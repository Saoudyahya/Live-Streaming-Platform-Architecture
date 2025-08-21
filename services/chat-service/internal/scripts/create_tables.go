// services/chat-service/scripts/create_tables.go
package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func main() {
	// Create AWS session for DynamoDB Local
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String("us-west-2"),
		Endpoint: aws.String("http://localhost:8000"), // DynamoDB Local endpoint
		Credentials: credentials.NewStaticCredentials(
			"fakeAccessKeyId",
			"fakeSecretAccessKey",
			"",
		),
	})
	if err != nil {
		log.Fatalf("Failed to create AWS session: %v", err)
	}

	db := dynamodb.New(sess)

	// Create Chatrooms table
	createChatroomsTable(db)

	// Create Messages table
	createMessagesTable(db)

	fmt.Println("All tables created successfully!")
}

func createChatroomsTable(db *dynamodb.DynamoDB) {
	tableName := "chatrooms"

	// Check if table already exists
	_, err := db.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		fmt.Printf("Table %s already exists\n", tableName)
		return
	}

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

	_, err = db.CreateTable(input)
	if err != nil {
		log.Fatalf("Failed to create %s table: %v", tableName, err)
	}

	fmt.Printf("Created table %s\n", tableName)
}

func createMessagesTable(db *dynamodb.DynamoDB) {
	tableName := "messages"

	// Check if table already exists
	_, err := db.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		fmt.Printf("Table %s already exists\n", tableName)
		return
	}

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

	_, err = db.CreateTable(input)
	if err != nil {
		log.Fatalf("Failed to create %s table: %v", tableName, err)
	}

	fmt.Printf("Created table %s\n", tableName)
}
