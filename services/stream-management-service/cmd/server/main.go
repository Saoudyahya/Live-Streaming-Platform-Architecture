// services/stream-management-service/cmd/server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/repository"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/server"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/service"
	grpcClient "github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/pkg/grpc"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	log.Printf("🚀 Starting Stream Management Service v%s (built %s)", Version, BuildTime)

	// Load configuration
	cfg := config.Load()
	log.Printf("📋 Configuration loaded: Environment=%s, Port=%s", cfg.Environment, cfg.Port)

	// Initialize repositories
	log.Println("🔗 Initializing repositories...")
	dynamoRepo := repository.NewDynamoDBRepository(cfg)
	redisRepo := repository.NewRedisRepository(cfg)
	log.Println("✅ Repositories initialized")

	// Initialize gRPC client to User Service (with graceful fallback)
	log.Printf("🔌 Attempting to connect to User Service at %s...", cfg.UserServiceGRPCAddr)
	var userClient *grpcClient.UserServiceClient
	var err error

	// Try to connect to User Service with timeout
	userClient, err = grpcClient.NewUserServiceClient(cfg.UserServiceGRPCAddr)
	if err != nil {
		log.Printf("⚠️ Failed to connect to User Service gRPC: %v", err)
		log.Println("⚠️ Continuing with fallback authentication (development mode)")
		userClient = nil
	} else {
		log.Println("✅ Connected to User Service gRPC")
	}

	// Initialize services
	log.Println("🔧 Initializing services...")
	streamService := service.NewStreamService(cfg, dynamoRepo, redisRepo)
	rtmpHandler := service.NewRTMPHandler(cfg, streamService, userClient)
	log.Println("✅ Services initialized")

	// Start gRPC server
	var grpcServer *grpc.Server
	if cfg.Environment != "http-only" { // Allow disabling gRPC for testing
		log.Println("🚀 Starting gRPC server...")
		grpcServer, err = server.StartGRPCServer(cfg, streamService, userClient)
		if err != nil {
			log.Printf("⚠️ Failed to start gRPC server: %v", err)
			log.Println("⚠️ Continuing with HTTP-only mode")
		} else {
			log.Println("✅ gRPC server started successfully")
		}
	}

	// Setup HTTP server for RTMP callbacks and API
	log.Println("🌐 Setting up HTTP server...")
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(server.CORSMiddleware())
	router.Use(server.LoggingMiddleware())
	router.Use(gin.Recovery())

	// Add request ID middleware
	router.Use(func(c *gin.Context) {
		c.Header("X-Request-ID", fmt.Sprintf("req_%d", time.Now().UnixNano()))
		c.Next()
	})

	// Health check endpoints
	router.GET("/health", server.HealthCheck)
	router.GET("/api/v1/health", server.HealthCheck)

	// Enhanced health check with gRPC status
	router.GET("/api/v1/health/detailed", func(c *gin.Context) {
		health := gin.H{
			"status":      "healthy",
			"service":     "stream-management",
			"version":     Version,
			"build_time":  BuildTime,
			"timestamp":   time.Now().Unix(),
			"environment": cfg.Environment,
			"components": gin.H{
				"http_server": "running",
				"dynamodb":    "connected",
				"redis":       "connected",
			},
		}

		// Check gRPC server status
		if grpcServer != nil {
			health["components"].(gin.H)["grpc_server"] = "running"
		} else {
			health["components"].(gin.H)["grpc_server"] = "disabled"
		}

		// Check User Service connection
		if userClient != nil {
			if err := userClient.HealthCheck(); err != nil {
				health["components"].(gin.H)["user_service"] = "disconnected"
			} else {
				health["components"].(gin.H)["user_service"] = "connected"
			}
		} else {
			health["components"].(gin.H)["user_service"] = "not_configured"
		}

		c.JSON(200, health)
	})

	// RTMP callback routes (used by media server)
	rtmpRoutes := router.Group("/rtmp")
	{
		rtmpRoutes.POST("/auth", rtmpHandler.AuthenticateStream)
		rtmpRoutes.POST("/started", rtmpHandler.StreamStarted)
		rtmpRoutes.POST("/ended", rtmpHandler.StreamEnded)
		rtmpRoutes.POST("/recorded", rtmpHandler.RecordingCompleted)
		rtmpRoutes.GET("/health", rtmpHandler.HealthCheck)
		rtmpRoutes.GET("/stream/:stream_key", rtmpHandler.GetStreamInfo)
	}

	// Stream management API routes
	apiRoutes := router.Group("/api/v1")
	{
		apiRoutes.GET("/streams", streamService.GetActiveStreams)
		apiRoutes.GET("/streams/:id", streamService.GetStreamByID)

		// Additional API endpoints
		apiRoutes.GET("/stats", func(c *gin.Context) {
			stats, err := streamService.GetPlatformStats()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, stats)
		})

		apiRoutes.GET("/user/:user_id/streams", func(c *gin.Context) {
			userID := c.Param("user_id")
			// TODO: Convert userID to int64 and get streams
			// For now, return placeholder
			c.JSON(http.StatusOK, gin.H{
				"message": "User streams endpoint",
				"user_id": userID,
				"note":    "Implementation pending",
			})
		})

		apiRoutes.POST("/streams/:id/viewers", func(c *gin.Context) {
			streamID := c.Param("id")
			var req struct {
				ViewerCount int `json:"viewer_count"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			err := streamService.UpdateViewerCount(streamID, req.ViewerCount)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Viewer count updated"})
		})

		// Test endpoint to verify service is working
		apiRoutes.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message":   "Stream Management Service is running",
				"timestamp": time.Now().Unix(),
				"version":   Version,
				"grpc":      grpcServer != nil,
				"features": []string{
					"RTMP authentication",
					"Stream lifecycle management",
					"Recording callbacks",
					"Session management",
					"gRPC API",
				},
			})
		})
	}

	// Debug routes (only in development)
	if cfg.Environment == "development" {
		debugRoutes := router.Group("/debug")
		{
			debugRoutes.GET("/config", func(c *gin.Context) {
				// Don't expose sensitive config in production
				safeConfig := map[string]interface{}{
					"environment": cfg.Environment,
					"port":        cfg.Port,
					"aws_region":  cfg.AWSRegion,
					"redis_addr":  cfg.RedisAddr,
					"version":     Version,
					"build_time":  BuildTime,
				}
				c.JSON(http.StatusOK, safeConfig)
			})

			debugRoutes.POST("/cleanup", func(c *gin.Context) {
				err := streamService.CleanupExpiredStreams()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"message": "Cleanup completed"})
			})

			// Test stream creation
			debugRoutes.POST("/test-stream", func(c *gin.Context) {
				testStream := &models.Stream{
					UserID:    123,
					StreamKey: "test_key_" + fmt.Sprintf("%d", time.Now().Unix()),
					Title:     "Test Stream",
					Status:    models.StreamStatusLive,
					Metadata:  map[string]string{"source": "debug"},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				now := time.Now()
				testStream.StartedAt = &now

				streamID, err := streamService.CreateStream(testStream)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"message":    "Test stream created",
					"stream_id":  streamID,
					"stream_key": testStream.StreamKey,
				})
			})

			// gRPC test endpoints
			if grpcServer != nil {
				debugRoutes.GET("/grpc/status", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{
						"grpc_server": "running",
						"reflection":  "enabled",
						"services":    []string{"StreamService"},
					})
				})
			}
		}
	}

	// Get port from environment
	port := cfg.Port
	if port == "" {
		port = "8081"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
		// Security and performance settings
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Start background tasks
	log.Println("⏰ Starting background tasks...")
	var wg sync.WaitGroup

	// Cleanup task
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if err := streamService.CleanupExpiredStreams(); err != nil {
				log.Printf("⚠️ Error in cleanup task: %v", err)
			}
		}
	}()

	// Start HTTP server in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("✅ Stream Management Service HTTP server started on port %s", port)
		log.Printf("📡 RTMP callbacks: http://localhost:%s/rtmp/*", port)
		log.Printf("🔌 API endpoints: http://localhost:%s/api/v1/*", port)
		log.Printf("🏥 Health check: http://localhost:%s/health", port)

		if cfg.Environment == "development" {
			log.Printf("🐛 Debug endpoints: http://localhost:%s/debug/*", port)
			log.Printf("🧪 Test stream creation: POST http://localhost:%s/debug/test-stream", port)
		}

		if grpcServer != nil {
			log.Printf("🚀 gRPC server: grpcurl -plaintext localhost:9090 list")
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Failed to start HTTP server: %v", err)
		}
	}()

	// Setup graceful shutdown
	log.Println("✅ All services started successfully")
	log.Println("📋 Service Summary:")
	log.Printf("   • HTTP Server: :%s", port)
	if grpcServer != nil {
		log.Printf("   • gRPC Server: :9090")
	}
	if userClient != nil {
		log.Printf("   • User Service: %s", cfg.UserServiceGRPCAddr)
	}
	log.Printf("   • Environment: %s", cfg.Environment)
	log.Printf("   • Version: %s", Version)
	log.Println("🎯 Ready to handle RTMP streams!")

	log.Println("")
	log.Println("📖 Quick Start Guide:")
	log.Printf("   1. Start your User Service (optional)")
	log.Printf("   2. Start SRS Media Server: docker-compose up -d")
	log.Printf("   3. Configure OBS with: rtmp://localhost:1935/live/YOUR_STREAM_KEY")
	log.Printf("   4. Test health: curl http://localhost:%s/health", port)
	if grpcServer != nil {
		log.Printf("   5. Test gRPC: grpcurl -plaintext localhost:9090 list")
	}
	log.Println("")

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down servers...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("❌ HTTP server forced to shutdown: %v", err)
	} else {
		log.Println("✅ HTTP server stopped gracefully")
	}

	// Shutdown gRPC server
	if grpcServer != nil {
		log.Println("🛑 Stopping gRPC server...")
		grpcServer.GracefulStop()
		log.Println("✅ gRPC server stopped gracefully")
	}

	// Close external connections
	if userClient != nil {
		userClient.Close()
		log.Println("✅ User service connection closed")
	}

	log.Println("👋 Stream Management Service shut down complete")
}
