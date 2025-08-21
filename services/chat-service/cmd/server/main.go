// services/chat-service/cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	chatpb "github.com/yourcompany/chat-app/gen/chat"
	userpb "github.com/yourcompany/chat-app/gen/user"
	"github.com/yourcompany/chat-app/services/chat-service/internal/config"
	"github.com/yourcompany/chat-app/services/chat-service/internal/repository"
	"github.com/yourcompany/chat-app/services/chat-service/internal/server"
	"github.com/yourcompany/chat-app/services/chat-service/internal/service"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize repositories
	dynamoRepo, err := repository.NewDynamoDBRepository(cfg.DynamoDB)
	if err != nil {
		log.Fatalf("Failed to initialize DynamoDB repository: %v", err)
	}

	redisRepo, err := repository.NewRedisRepository(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to initialize Redis repository: %v", err)
	}

	// Initialize user service client
	userConn, err := grpc.Dial(cfg.UserService.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to user service: %v", err)
	}
	defer userConn.Close()

	userClient := userpb.NewUserServiceClient(userConn)

	// Initialize chat service
	chatService := service.NewChatService(dynamoRepo, redisRepo, userClient)

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(server.LoggingInterceptor),
	)
	chatpb.RegisterChatServiceServer(grpcServer, chatService)

	// Enable gRPC reflection for development
	reflection.Register(grpcServer)

	// Create WebSocket hub
	wsHub := server.NewWebSocketHub()
	go wsHub.Run()

	// Initialize WebSocket handler
	wsHandler := service.NewWebSocketHandler(chatService, wsHub, userClient)

	// Setup HTTP server for WebSocket connections
	router := mux.NewRouter()
	router.HandleFunc("/ws", wsHandler.HandleWebSocket)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:    cfg.Server.HTTPPort,
		Handler: router,
	}

	// Start servers
	go func() {
		log.Printf("Starting gRPC server on %s", cfg.Server.GRPCPort)
		lis, err := net.Listen("tcp", cfg.Server.GRPCPort)
		if err != nil {
			log.Fatalf("Failed to listen on gRPC port: %v", err)
		}
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting HTTP server on %s", cfg.Server.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced to shutdown: %v", err)
	}

	grpcServer.GracefulStop()
	wsHub.Close()

	log.Println("Servers stopped")
}
