// services/stream-management-service/internal/repository/dynamodb.go
package repository

import (
	"fmt"
	"log"
	_ "os"
	_ "time"

	"github.com/aws/aws-sdk-go/aws"
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
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(cfg.AWSRegion),
	}))

	return &DynamoDBRepository{
		client:    dynamodb.New(sess),
		tableName: cfg.DynamoDBTableName,
	}
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
