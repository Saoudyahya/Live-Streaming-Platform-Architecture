// services/stream-management-service/cmd/server/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	_ "strconv"
	"syscall"
	"time"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/repository"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/server"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/service"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/grpc"
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("🚀 Starting Stream Management Service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("📋 Configuration loaded: %+v", cfg)

	// Initialize repositories
	dynamoRepo := repository.NewDynamoDBRepository(cfg)
	redisRepo := repository.NewRedisRepository(cfg)

	// Initialize gRPC clients
	userClient, err := grpc.NewUserServiceClient(cfg.UserServiceGRPCAddr)
	if err != nil {
		log.Fatalf("❌ Failed to connect to User Service: %v", err)
	}
	defer userClient.Close()

	// Initialize services
	streamService := service.NewStreamService(cfg, dynamoRepo, redisRepo)
	rtmpHandler := service.NewRTMPHandler(cfg, streamService, userClient)

	// Setup HTTP server for RTMP callbacks
	router := gin.Default()

	// Add middleware
	router.Use(server.CORSMiddleware())
	router.Use(server.LoggingMiddleware())

	// RTMP callback routes
	rtmpRoutes := router.Group("/rtmp")
	{
		rtmpRoutes.POST("/auth", rtmpHandler.AuthenticateStream)
		rtmpRoutes.POST("/started", rtmpHandler.StreamStarted)
		rtmpRoutes.POST("/ended", rtmpHandler.StreamEnded)
		rtmpRoutes.POST("/recorded", rtmpHandler.RecordingCompleted)
	}

	// Management routes
	apiRoutes := router.Group("/api/v1")
	{
		apiRoutes.GET("/health", server.HealthCheck)
		apiRoutes.GET("/streams", streamService.GetActiveStreams)
		apiRoutes.GET("/streams/:id", streamService.GetStreamByID)
	}

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("✅ Stream Management Service started on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("❌ Server forced to shutdown:", err)
	}

	log.Println("✅ Server exited")
}
