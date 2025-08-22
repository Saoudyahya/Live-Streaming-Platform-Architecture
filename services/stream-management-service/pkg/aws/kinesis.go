// services/stream-management-service/pkg/aws/kinesis.go
package aws

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

type KinesisClient struct {
	client     *kinesis.Kinesis
	streamName string
}

func NewKinesisClient(region, streamName string) *KinesisClient {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))

	return &KinesisClient{
		client:     kinesis.New(sess),
		streamName: streamName,
	}
}

func (k *KinesisClient) PutRecord(data string) error {
	input := &kinesis.PutRecordInput{
		Data:         []byte(data),
		PartitionKey: aws.String("default"),
		StreamName:   aws.String(k.streamName),
	}

	result, err := k.client.PutRecord(input)
	if err != nil {
		return fmt.Errorf("failed to put record to Kinesis: %w", err)
	}

	log.Printf("✅ Event published to Kinesis: %s", *result.SequenceNumber)
	return nil
}
