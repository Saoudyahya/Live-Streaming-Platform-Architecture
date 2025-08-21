// services/chat-service/cmd/server/main.go
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/migration"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/repository"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/server"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/service"
	chatpb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/pkg/proto/chat"
	userpb "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/pkg/proto/user"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Create AWS session for DynamoDB
	awsConfig := &aws.Config{
		Region: aws.String(cfg.DynamoDB.Region),
	}

	// Add credentials if provided
	if cfg.DynamoDB.AccessKeyID != "" && cfg.DynamoDB.SecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(
			cfg.DynamoDB.AccessKeyID,
			cfg.DynamoDB.SecretAccessKey,
			"",
		)
	}

	// For local development with DynamoDB Local
	if os.Getenv("DYNAMODB_ENDPOINT") != "" {
		awsConfig.Endpoint = aws.String(os.Getenv("DYNAMODB_ENDPOINT"))
		log.Printf("Using DynamoDB endpoint: %s", os.Getenv("DYNAMODB_ENDPOINT"))
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	dynamoClient := dynamodb.New(sess)

	// Run migrations to create tables
	migrator := migration.NewDynamoDBMigrator(dynamoClient, &cfg.DynamoDB)
	if err := migrator.CreateTables(); err != nil {
		log.Fatalf("Failed to create DynamoDB tables: %v", err)
	}

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

	log.Println("Chat service started successfully!")

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
