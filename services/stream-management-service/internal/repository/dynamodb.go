// services/stream-management-service/internal/repository/dynamodb.go
package repository

import (
	"fmt"
	"log"
	_ "os"
	_ "time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	_ "github.com/aws/aws-sdk-go/service/kinesis"
	_ "github.com/aws/aws-sdk-go/service/s3/s3manager"
	_ "github.com/gin-gonic/gin"
	_ "github.com/go-redis/redis/v8"
	_ "google.golang.org/grpc"
	_ "google.golang.org/grpc/credentials/insecure"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/config"
	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/stream-management-service/internal/models"
)

type DynamoDBRepository struct {
	client    *dynamodb.DynamoDB
	tableName string
}

func NewDynamoDBRepository(cfg *config.Config) *DynamoDBRepository {
	// Configure AWS session for local development
	var sess *session.Session
	var err error

	if cfg.Environment == "development" || cfg.DynamoDBEndpoint != "" {
		// Local DynamoDB configuration
		log.Printf("🔧 Configuring for local DynamoDB at: %s", cfg.DynamoDBEndpoint)

		sess, err = session.NewSession(&aws.Config{
			Region:      aws.String(cfg.AWSRegion),
			Endpoint:    aws.String(cfg.DynamoDBEndpoint),
			Credentials: credentials.NewStaticCredentials("dummy", "dummy", ""),
		})
	} else {
		// Production AWS configuration
		sess, err = session.NewSession(&aws.Config{
			Region: aws.String(cfg.AWSRegion),
		})
	}

	if err != nil {
		log.Fatalf("❌ Failed to create AWS session: %v", err)
	}

	dynamoClient := dynamodb.New(sess)

	// Create table if it doesn't exist (for local development)
	if cfg.Environment == "development" {
		if err := createTableIfNotExists(dynamoClient, cfg.DynamoDBTableName); err != nil {
			log.Printf("⚠️ Warning: Could not create/verify table: %v", err)
		} else {
			log.Printf("✅ DynamoDB table '%s' ready", cfg.DynamoDBTableName)
		}
	}

	return &DynamoDBRepository{
		client:    dynamoClient,
		tableName: cfg.DynamoDBTableName,
	}
}

// createTableIfNotExists creates the streams table if it doesn't exist
func createTableIfNotExists(client *dynamodb.DynamoDB, tableName string) error {
	// Check if table exists
	_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})

	if err == nil {
		// Table exists
		log.Printf("📋 Table '%s' already exists", tableName)
		return nil
	}

	// Table doesn't exist, create it
	log.Printf("🔨 Creating DynamoDB table: %s", tableName)

	input := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("HASH"), // Partition key
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"), // String
			},
			{
				AttributeName: aws.String("stream_key"),
				AttributeType: aws.String("S"), // String
			},
			{
				AttributeName: aws.String("status"),
				AttributeType: aws.String("S"), // String
			},
			{
				AttributeName: aws.String("user_id"),
				AttributeType: aws.String("N"), // Number
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"), // On-demand pricing
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			// GSI for querying by stream_key
			{
				IndexName: aws.String("stream-key-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("stream_key"),
						KeyType:       aws.String("HASH"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
			// GSI for querying by status
			{
				IndexName: aws.String("status-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("status"),
						KeyType:       aws.String("HASH"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
			// GSI for querying by user_id
			{
				IndexName: aws.String("user-id-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("user_id"),
						KeyType:       aws.String("HASH"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
		},
	}

	result, err := client.CreateTable(input)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	log.Printf("✅ Table created successfully: %s", *result.TableDescription.TableName)

	// Wait for table to be active
	log.Printf("⏳ Waiting for table to become active...")
	err = client.WaitUntilTableExists(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})

	if err != nil {
		return fmt.Errorf("failed to wait for table: %w", err)
	}

	log.Printf("🎉 Table '%s' is now active and ready!", tableName)
	return nil
}

func (r *DynamoDBRepository) CreateStream(stream *models.Stream) error {
	item, err := dynamodbattribute.MarshalMap(stream)
	if err != nil {
		return fmt.Errorf("failed to marshal stream: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}

	_, err = r.client.PutItem(input)
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	log.Printf("✅ Stream created in DynamoDB: %s", stream.ID)
	return nil
}

func (r *DynamoDBRepository) GetStreamByID(streamID string) (*models.Stream, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(streamID),
			},
		},
	}

	result, err := r.client.GetItem(input)
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("stream not found")
	}

	var stream models.Stream
	err = dynamodbattribute.UnmarshalMap(result.Item, &stream)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream: %w", err)
	}

	return &stream, nil
}

func (r *DynamoDBRepository) GetStreamByStreamKey(streamKey string) (*models.Stream, error) {
	// Use GSI for better performance
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("stream-key-index"),
		KeyConditionExpression: aws.String("stream_key = :stream_key"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":stream_key": {
				S: aws.String(streamKey),
			},
		},
		Limit: aws.Int64(1), // We only expect one result
	}

	result, err := r.client.Query(input)
	if err != nil {
		// Fallback to scan if GSI doesn't exist yet
		log.Printf("⚠️ GSI query failed, falling back to scan: %v", err)
		return r.getStreamByStreamKeyScan(streamKey)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("stream not found")
	}

	var stream models.Stream
	err = dynamodbattribute.UnmarshalMap(result.Items[0], &stream)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream: %w", err)
	}

	return &stream, nil
}

// Fallback scan method for when GSI is not available
func (r *DynamoDBRepository) getStreamByStreamKeyScan(streamKey string) (*models.Stream, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("stream_key = :stream_key"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":stream_key": {
				S: aws.String(streamKey),
			},
		},
	}

	result, err := r.client.Scan(input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan items: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("stream not found")
	}

	var stream models.Stream
	err = dynamodbattribute.UnmarshalMap(result.Items[0], &stream)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream: %w", err)
	}

	return &stream, nil
}

func (r *DynamoDBRepository) GetStreamsByStatus(status models.StreamStatus) ([]*models.Stream, error) {
	// Use GSI for better performance
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("status-index"),
		KeyConditionExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {
				S: aws.String(string(status)),
			},
		},
	}

	result, err := r.client.Query(input)
	if err != nil {
		// Fallback to scan if GSI doesn't exist yet
		log.Printf("⚠️ GSI query failed, falling back to scan: %v", err)
		return r.getStreamsByStatusScan(status)
	}

	var streams []*models.Stream
	for _, item := range result.Items {
		var stream models.Stream
		err = dynamodbattribute.UnmarshalMap(item, &stream)
		if err != nil {
			log.Printf("⚠️ Failed to unmarshal stream: %v", err)
			continue
		}
		streams = append(streams, &stream)
	}

	return streams, nil
}

// Fallback scan method for when GSI is not available
func (r *DynamoDBRepository) getStreamsByStatusScan(status models.StreamStatus) ([]*models.Stream, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {
				S: aws.String(string(status)),
			},
		},
	}

	result, err := r.client.Scan(input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan items: %w", err)
	}

	var streams []*models.Stream
	for _, item := range result.Items {
		var stream models.Stream
		err = dynamodbattribute.UnmarshalMap(item, &stream)
		if err != nil {
			log.Printf("⚠️ Failed to unmarshal stream: %v", err)
			continue
		}
		streams = append(streams, &stream)
	}

	return streams, nil
}

func (r *DynamoDBRepository) UpdateStream(stream *models.Stream) error {
	item, err := dynamodbattribute.MarshalMap(stream)
	if err != nil {
		return fmt.Errorf("failed to marshal stream: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}

	_, err = r.client.PutItem(input)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	log.Printf("✅ Stream updated in DynamoDB: %s", stream.ID)
	return nil
}
