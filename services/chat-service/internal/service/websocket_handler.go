package service

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/server"
	userpb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/pkg/proto/user"
)

type WebSocketHandler struct {
	chatService *ChatService
	hub         *server.Hub
	userClient  userpb.UserServiceClient
}

type WebSocketMessage struct {
	Type       string      `json:"type"`
	ChatroomID string      `json:"chatroom_id,omitempty"`
	Content    string      `json:"content,omitempty"`
	Data       interface{} `json:"data,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

func NewWebSocketHandler(chatService *ChatService, hub *server.Hub, userClient userpb.UserServiceClient) *WebSocketHandler {
	return &WebSocketHandler{
		chatService: chatService,
		hub:         hub,
		userClient:  userClient,
	}
}

func (h *WebSocketHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract user info from query parameters or headers
	// In production, validate JWT token
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Validate user exists
	userResp, err := h.userClient.GetUser(r.Context(), &userpb.GetUserRequest{
		UserId: userID,
	})
	if err != nil || !userResp.Status.Success {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &server.Client{
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      h.hub,
		UserID:   userID,
		Username: userResp.User.Username,
		Rooms:    make(map[string]bool),
	}

	// Register client using the hub's method
	h.hub.RegisterClient(client)

	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump()
}
