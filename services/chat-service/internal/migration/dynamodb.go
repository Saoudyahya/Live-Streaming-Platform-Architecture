// services/chat-service/internal/migration/dynamodb.go
package migration

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/Saoudyahya/Live-Streaming-Platform-Architecture/services/chat-service/internal/config"
)

type DynamoDBMigrator struct {
	db     *dynamodb.DynamoDB
	config *config.DynamoDBConfig
}

func NewDynamoDBMigrator(db *dynamodb.DynamoDB, cfg *config.DynamoDBConfig) *DynamoDBMigrator {
	return &DynamoDBMigrator{
		db:     db,
		config: cfg,
	}
}

func (m *DynamoDBMigrator) CreateTables() error {
	log.Println("Starting DynamoDB table creation...")

	// Create Chatrooms table
	if err := m.createChatroomsTable(); err != nil {
		return fmt.Errorf("failed to create chatrooms table: %w", err)
	}

	// Create Messages table
	if err := m.createMessagesTable(); err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	log.Println("All DynamoDB tables created successfully!")
	return nil
}

func (m *DynamoDBMigrator) createChatroomsTable() error {
	tableName := m.config.ChatroomTable

	// Check if table already exists
	_, err := m.db.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		log.Printf("Table %s already exists, skipping creation", tableName)
		return nil
	}

	log.Printf("Creating table %s...", tableName)

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
				AttributeName: aws.String("creator_id"),
				AttributeType: aws.String("S"), // String
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String("creator-id-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("creator_id"),
						KeyType:       aws.String("HASH"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
		},
	}

	_, err = m.db.CreateTable(input)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	// Wait for table to be active
	return m.waitForTableActive(tableName)
}

func (m *DynamoDBMigrator) createMessagesTable() error {
	tableName := m.config.MessageTable

	// Check if table already exists
	_, err := m.db.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		log.Printf("Table %s already exists, skipping creation", tableName)
		return nil
	}

	log.Printf("Creating table %s...", tableName)

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
				AttributeName: aws.String("chatroom_id"),
				AttributeType: aws.String("S"), // String
			},
			{
				AttributeName: aws.String("created_at"),
				AttributeType: aws.String("S"), // String (ISO timestamp)
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String("chatroom-created-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("chatroom_id"),
						KeyType:       aws.String("HASH"),
					},
					{
						AttributeName: aws.String("created_at"),
						KeyType:       aws.String("RANGE"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
		},
	}

	_, err = m.db.CreateTable(input)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	// Wait for table to be active
	return m.waitForTableActive(tableName)
}

func (m *DynamoDBMigrator) waitForTableActive(tableName string) error {
	log.Printf("Waiting for table %s to become active...", tableName)

	maxRetries := 30
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		resp, err := m.db.DescribeTable(&dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			return fmt.Errorf("failed to describe table %s: %w", tableName, err)
		}

		if *resp.Table.TableStatus == "ACTIVE" {
			log.Printf("Table %s is now active", tableName)
			return nil
		}

		log.Printf("Table %s status: %s, waiting...", tableName, *resp.Table.TableStatus)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("table %s did not become active within timeout", tableName)
}

// Add this method to your DynamoDBMigrator struct in migration/dynamodb.go

func (m *DynamoDBMigrator) ForceCleanup() error {
	log.Println("🧹 Force cleaning up all tables...")

	tables := []string{m.config.ChatroomTable, m.config.MessageTable}

	for _, tableName := range tables {
		log.Printf("Attempting to delete table: %s", tableName)

		// Try to delete the table (ignore errors if table doesn't exist)
		_, err := m.db.DeleteTable(&dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})

		if err != nil {
			log.Printf("Note: Could not delete table %s (might not exist): %v", tableName, err)
		} else {
			log.Printf("✅ Table %s deletion initiated", tableName)

			// Wait for table to be deleted
			log.Printf("Waiting for table %s to be fully deleted...", tableName)
			err = m.db.WaitUntilTableNotExists(&dynamodb.DescribeTableInput{
				TableName: aws.String(tableName),
			})

			if err != nil {
				log.Printf("Warning: Error waiting for table %s deletion: %v", tableName, err)
			} else {
				log.Printf("✅ Table %s fully deleted", tableName)
			}
		}
	}

	log.Println("✅ Force cleanup completed!")
	return nil
}

// Also add this method to always recreate tables
func (m *DynamoDBMigrator) ForceCreateTables() error {
	log.Println("🚀 Force creating tables (will recreate if they exist)...")

	// First cleanup
	m.ForceCleanup()

	// Then create fresh tables
	return m.CreateTables()
}
