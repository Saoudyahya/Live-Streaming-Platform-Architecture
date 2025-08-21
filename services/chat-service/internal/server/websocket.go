package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin in development
		// In production, implement proper origin checking
		return true
	},
}

// Client represents a WebSocket client
type Client struct {
	Conn     *websocket.Conn // Exported
	Send     chan []byte     // Exported
	Hub      *Hub            // Exported
	UserID   string          // Exported
	Username string          // Exported
	Rooms    map[string]bool // Exported
}

// Hub maintains active WebSocket connections
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	rooms      map[string]map[*Client]bool
	mutex      sync.RWMutex
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
	}
}

// Run starts the WebSocket hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// Close gracefully shuts down the hub
func (h *Hub) Close() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for client := range h.clients {
		close(client.Send)
		client.Conn.Close()
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.clients[client] = true
	log.Printf("Client registered: %s (%s)", client.Username, client.UserID)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.Send)

		// Remove from all rooms
		for roomID := range client.Rooms {
			if room, exists := h.rooms[roomID]; exists {
				delete(room, client)
				if len(room) == 0 {
					delete(h.rooms, roomID)
				}
			}
		}

		log.Printf("Client unregistered: %s (%s)", client.Username, client.UserID)
	}
}

func (h *Hub) broadcastMessage(message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(h.clients, client)
		}
	}
}

// JoinRoom adds a client to a specific chat room
func (h *Hub) JoinRoom(client *Client, roomID string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]bool)
	}

	h.rooms[roomID][client] = true
	client.Rooms[roomID] = true

	log.Printf("Client %s joined room %s", client.Username, roomID)
}

// LeaveRoom removes a client from a specific chat room
func (h *Hub) LeaveRoom(client *Client, roomID string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if room, exists := h.rooms[roomID]; exists {
		delete(room, client)
		if len(room) == 0 {
			delete(h.rooms, roomID)
		}
	}

	delete(client.Rooms, roomID)

	log.Printf("Client %s left room %s", client.Username, roomID)
}

// BroadcastToRoom sends a message to all clients in a specific room
func (h *Hub) BroadcastToRoom(roomID string, message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if room, exists := h.rooms[roomID]; exists {
		for client := range room {
			select {
			case client.Send <- message:
			default:
				close(client.Send)
				delete(h.clients, client)
				delete(room, client)
			}
		}
	}
}

// RegisterClient registers a new client with the hub
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient unregisters a client from the hub
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// ReadPump handles messages from the WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.UnregisterClient(c)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming message
		log.Printf("Received message from %s: %s", c.Username, string(message))

		// Echo message back to the room (simplified)
		// In practice, you'd parse the message and handle different types
		c.Hub.Broadcast(message)
	}
}

// WritePump handles messages to the WebSocket connection
func (c *Client) WritePump() {
	defer c.Conn.Close()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}
