// // services/stream-management-service/internal/config/config.go
// package config
//
// import (
//
//	"os"
//	"strconv"
//	"time"
//
// )
//
//	type Config struct {
//		// Server
//		Port        string
//		Environment string
//
//		// External Services
//		UserServiceGRPCAddr string
//
//		// AWS
//		AWSRegion         string
//		DynamoDBTableName string
//		KinesisStreamName string
//		S3BucketName      string
//
//		// Redis
//		RedisAddr     string
//		RedisPassword string
//		RedisDB       int
//
//		// Timeouts
//		HTTPTimeout time.Duration
//		GRPCTimeout time.Duration
//	}
//
//	func Load() *Config {
//		return &Config{
//			// Server
//			Port:        getEnv("PORT", "8081"),
//			Environment: getEnv("ENVIRONMENT", "development"),
//
//			// External Services
//			UserServiceGRPCAddr: getEnv("USER_SERVICE_GRPC_ADDR", "user-service:8083"),
//
//			// AWS
//			AWSRegion:         getEnv("AWS_REGION", "us-east-1"),
//			DynamoDBTableName: getEnv("DYNAMODB_TABLE_NAME", "streams"),
//			KinesisStreamName: getEnv("KINESIS_STREAM_NAME", "stream-events"),
//			S3BucketName:      getEnv("S3_BUCKET_NAME", "stream-recordings"),
//
//			// Redis
//			RedisAddr:     getEnv("REDIS_ADDR", "redis:6379"),
//			RedisPassword: getEnv("REDIS_PASSWORD", ""),
//			RedisDB:       getEnvAsInt("REDIS_DB", 0),
//
//			// Timeouts
//			HTTPTimeout: getEnvAsDuration("HTTP_TIMEOUT", 30*time.Second),
//			GRPCTimeout: getEnvAsDuration("GRPC_TIMEOUT", 10*time.Second),
//		}
//	}
//
//	func getEnv(key, defaultValue string) string {
//		if value := os.Getenv(key); value != "" {
//			return value
//		}
//		return defaultValue
//	}
//
//	func getEnvAsInt(key string, defaultValue int) int {
//		if value := os.Getenv(key); value != "" {
//			if intValue, err := strconv.Atoi(value); err == nil {
//				return intValue
//			}
//		}
//		return defaultValue
//	}
//
//	func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
//		if value := os.Getenv(key); value != "" {
//			if duration, err := time.ParseDuration(value); err == nil {
//				return duration
//			}
//		}
//		return defaultValue
//	}
//
// services/stream-management-service/internal/config/config.go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port        string
	Environment string

	// External Services
	UserServiceGRPCAddr string

	// AWS / DynamoDB
	AWSRegion         string
	DynamoDBTableName string
	DynamoDBEndpoint  string // Added for local DynamoDB
	KinesisStreamName string
	S3BucketName      string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Timeouts
	HTTPTimeout time.Duration
	GRPCTimeout time.Duration
}

func Load() *Config {
	return &Config{
		// Server
		Port:        getEnv("PORT", "8084"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// External Services
		UserServiceGRPCAddr: getEnv("USER_SERVICE_GRPC_ADDR", "user-service:8082"),

		// AWS / DynamoDB
		AWSRegion:         getEnv("AWS_REGION", "us-east-1"),
		DynamoDBTableName: getEnv("DYNAMODB_TABLE_NAME", "streams"),
		DynamoDBEndpoint:  getEnv("DYNAMODB_ENDPOINT", "http://localhost:8002"), // Local DynamoDB
		KinesisStreamName: getEnv("KINESIS_STREAM_NAME", "stream-events"),
		S3BucketName:      getEnv("S3_BUCKET_NAME", "stream-recordings"),

		// Redis - Updated to match your docker-compose
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"), // Changed from redis:6379 for local dev
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		// Timeouts
		HTTPTimeout: getEnvAsDuration("HTTP_TIMEOUT", 30*time.Second),
		GRPCTimeout: getEnvAsDuration("GRPC_TIMEOUT", 10*time.Second),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
