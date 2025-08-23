// services/chat-service/cmd/server/main.go
package main

import (
	"context"
	"flag"
	"fmt"
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

// Enhanced cleanup functionality
func forceCleanupTables(client *dynamodb.DynamoDB, cfg *config.DynamoDBConfig) error {
	log.Println("🧹 Force cleaning up all tables...")

	tables := []string{cfg.ChatroomTable, cfg.MessageTable}

	for _, tableName := range tables {
		log.Printf("Attempting to delete table: %s", tableName)

		// Check if table exists first
		_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})

		if err != nil {
			log.Printf("Table %s doesn't exist, skipping deletion", tableName)
			continue
		}

		// Try to delete the table
		_, err = client.DeleteTable(&dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})

		if err != nil {
			log.Printf("Error deleting table %s: %v", tableName, err)
			continue
		}

		log.Printf("✅ Table %s deletion initiated", tableName)

		// Wait for table to be deleted with timeout
		log.Printf("Waiting for table %s to be fully deleted...", tableName)
		maxWait := 60 // seconds
		for i := 0; i < maxWait; i++ {
			_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
				TableName: aws.String(tableName),
			})

			if err != nil {
				// Table no longer exists
				log.Printf("✅ Table %s fully deleted", tableName)
				break
			}

			if i == maxWait-1 {
				log.Printf("⚠️  Timeout waiting for table %s deletion", tableName)
			}

			time.Sleep(1 * time.Second)
		}
	}

	log.Println("✅ Force cleanup completed!")
	return nil
}

// List all tables for debugging
func listTables(client *dynamodb.DynamoDB) error {
	log.Println("📋 Listing all current tables...")

	result, err := client.ListTables(&dynamodb.ListTablesInput{})
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	if len(result.TableNames) == 0 {
		log.Println("  No tables found")
	} else {
		log.Printf("  Found %d tables:", len(result.TableNames))
		for _, table := range result.TableNames {
			log.Printf("    - %s", *table)
		}
	}

	return nil
}

// Wait for DynamoDB to be ready
func waitForDynamoDB(client *dynamodb.DynamoDB, maxRetries int) error {
	log.Printf("⏳ Waiting for DynamoDB to be ready...")
	for i := 0; i < maxRetries; i++ {
		_, err := client.ListTables(&dynamodb.ListTablesInput{})
		if err == nil {
			log.Printf("✅ DynamoDB is ready after %d attempts", i+1)
			return nil
		}
		log.Printf("  Attempt %d/%d failed: %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("DynamoDB not ready after %d attempts", maxRetries)
}

func main() {
	// Parse command line flags
	var (
		forceCleanup = flag.Bool("force-cleanup", false, "Force delete and recreate all tables")
		cleanup      = flag.Bool("cleanup", false, "Delete existing tables before starting")
		listOnly     = flag.Bool("list-tables", false, "List all tables and exit")
		skipTables   = flag.Bool("skip-tables", false, "Skip table creation/migration")
		verbose      = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Println("🚀 Starting Chat Service...")

	// Display startup configuration
	if *forceCleanup {
		log.Println("🧹 Mode: Force cleanup and recreate tables")
	} else if *cleanup {
		log.Println("🧹 Mode: Cleanup existing tables")
	} else if *listOnly {
		log.Println("📋 Mode: List tables only")
	} else if *skipTables {
		log.Println("⏭️  Mode: Skip table operations")
	} else {
		log.Println("🔄 Mode: Normal startup")
	}

	// Load configuration
	cfg := config.Load()
	log.Printf("📁 Configuration loaded: Region=%s, Tables=[%s, %s]",
		cfg.DynamoDB.Region, cfg.DynamoDB.ChatroomTable, cfg.DynamoDB.MessageTable)

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
		log.Println("🔑 Using provided AWS credentials")
	}

	// For local development with DynamoDB Local
	if os.Getenv("DYNAMODB_ENDPOINT") != "" {
		awsConfig.Endpoint = aws.String(os.Getenv("DYNAMODB_ENDPOINT"))
		log.Printf("🏠 Using DynamoDB endpoint: %s", os.Getenv("DYNAMODB_ENDPOINT"))
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		log.Fatalf("❌ Failed to create AWS session: %v", err)
	}

	dynamoClient := dynamodb.New(sess)

	// Wait for DynamoDB to be ready
	if err := waitForDynamoDB(dynamoClient, 30); err != nil {
		log.Fatalf("❌ DynamoDB not available: %v", err)
	}

	// Handle list-only mode
	if *listOnly {
		if err := listTables(dynamoClient); err != nil {
			log.Fatalf("❌ Failed to list tables: %v", err)
		}
		return
	}

	// Handle table operations
	if !*skipTables {
		// Handle cleanup operations
		if *forceCleanup || *cleanup {
			if err := forceCleanupTables(dynamoClient, &cfg.DynamoDB); err != nil {
				log.Fatalf("❌ Failed to cleanup tables: %v", err)
			}
		}

		// Create tables (unless we're only cleaning up)
		if !*cleanup || *forceCleanup {
			log.Println("🏗️  Creating/checking database tables...")
			migrator := migration.NewDynamoDBMigrator(dynamoClient, &cfg.DynamoDB)
			if err := migrator.CreateTables(); err != nil {
				log.Fatalf("❌ Failed to create DynamoDB tables: %v", err)
			}
		}

		// List tables after operations for verification
		if *verbose {
			if err := listTables(dynamoClient); err != nil {
				log.Printf("⚠️  Failed to list tables: %v", err)
			}
		}
	}

	// If we're only doing cleanup, exit here
	if *cleanup && !*forceCleanup {
		log.Println("✅ Cleanup completed. Exiting.")
		return
	}

	// Initialize repositories
	log.Println("🔧 Initializing repositories...")
	dynamoRepo, err := repository.NewDynamoDBRepository(cfg.DynamoDB)
	if err != nil {
		log.Fatalf("❌ Failed to initialize DynamoDB repository: %v", err)
	}

	redisRepo, err := repository.NewRedisRepository(cfg.Redis)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Redis repository: %v", err)
	}

	// Initialize user service client
	log.Printf("🔗 Connecting to user service at %s...", cfg.UserService.Address)
	userConn, err := grpc.Dial(cfg.UserService.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("❌ Failed to connect to user service: %v", err)
	}
	defer userConn.Close()

	userClient := userpb.NewUserServiceClient(userConn)

	// Initialize chat service
	log.Println("💬 Initializing chat service...")
	chatService := service.NewChatService(dynamoRepo, redisRepo, userClient)

	// Create gRPC server with enhanced setup
	log.Println("🔧 Setting up gRPC server with reflection...")
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(server.LoggingInterceptor),
		// Add any additional interceptors here if needed
		grpc.MaxRecvMsgSize(4*1024*1024), // 4MB max message size
		grpc.MaxSendMsgSize(4*1024*1024), // 4MB max message size
	)

	// Register the chat service
	chatpb.RegisterChatServiceServer(grpcServer, chatService)

	// IMPORTANT: Enable gRPC reflection for development and debugging
	reflection.Register(grpcServer)
	log.Println("✅ gRPC reflection enabled - Postman should now work!")

	// Create WebSocket hub
	log.Println("🌐 Setting up WebSocket hub...")
	wsHub := server.NewWebSocketHub()
	go wsHub.Run()

	// Initialize WebSocket handler
	wsHandler := service.NewWebSocketHandler(chatService, wsHub, userClient)

	// Setup HTTP server for WebSocket connections
	log.Println("🔧 Setting up HTTP server...")
	router := mux.NewRouter()
	router.HandleFunc("/ws", wsHandler.HandleWebSocket)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "chat-service", "grpc_reflection": "enabled"}`))
	})

	httpServer := &http.Server{
		Addr:    cfg.Server.HTTPPort,
		Handler: router,
		// Add timeouts for better reliability
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start servers
	go func() {
		log.Printf("🚀 Starting gRPC server on %s", cfg.Server.GRPCPort)
		lis, err := net.Listen("tcp", cfg.Server.GRPCPort)
		if err != nil {
			log.Fatalf("❌ Failed to listen on gRPC port: %v", err)
		}

		log.Printf("✅ gRPC server listening on %s with reflection enabled", cfg.Server.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("❌ Failed to serve gRPC: %v", err)
		}
	}()

	go func() {
		log.Printf("🚀 Starting HTTP server on %s", cfg.Server.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Failed to start HTTP server: %v", err)
		}
	}()

	log.Println("✅ Chat service started successfully!")
	log.Printf("📡 gRPC server: localhost%s (reflection enabled)", cfg.Server.GRPCPort)
	log.Printf("🌐 HTTP server: localhost%s", cfg.Server.HTTPPort)
	log.Printf("🔗 WebSocket: ws://localhost%s/ws", cfg.Server.HTTPPort)
	log.Println("💡 Use Ctrl+C to gracefully shut down")
	log.Println("🔍 Postman should now be able to load gRPC reflection!")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("⚠️  HTTP server forced to shutdown: %v", err)
	}

	grpcServer.GracefulStop()
	wsHub.Close()

	log.Println("✅ Servers stopped gracefully")
}
