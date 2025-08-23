//// services/stream-management-service/pkg/aws/s3.go
//package aws
//
//import (
//	"fmt"
//	"os"
//
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/session"
//	_ "github.com/aws/aws-sdk-go/service/s3"
//	"github.com/aws/aws-sdk-go/service/s3/s3manager"
//)
//
//type S3Client struct {
//	uploader   *s3manager.Uploader
//	bucketName string
//}
//
//func NewS3Client(region, bucketName string) *S3Client {
//	sess := session.Must(session.NewSession(&aws.Config{
//		Region: aws.String(region),
//	}))
//
//	return &S3Client{
//		uploader:   s3manager.NewUploader(sess),
//		bucketName: bucketName,
//	}
//}
//
//func (s *S3Client) UploadRecording(filePath, key string) (string, error) {
//	file, err := os.Open(filePath)
//	if err != nil {
//		return "", fmt.Errorf("failed to open file: %w", err)
//	}
//	defer file.Close()
//
//	result, err := s.uploader.Upload(&s3manager.UploadInput{
//		Bucket: aws.String(s.bucketName),
//		Key:    aws.String(key),
//		Body:   file,
//	})
//	if err != nil {
//		return "", fmt.Errorf("failed to upload to S3: %w", err)
//	}
//
//	return result.Location, nil
//}

// services/stream-management-service/pkg/aws/s3.go
package aws

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	_ "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3Client struct {
	uploader   *s3manager.Uploader
	bucketName string
	mockMode   bool
}

func NewS3Client(region, bucketName string) *S3Client {
	// Check if we're in development mode
	env := os.Getenv("ENVIRONMENT")
	mockMode := env == "development" || env == ""

	if mockMode {
		log.Printf("🔧 S3 client running in mock mode (development)")
		return &S3Client{
			uploader:   nil,
			bucketName: bucketName,
			mockMode:   true,
		}
	}

	// Production mode - use real S3
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))

	return &S3Client{
		uploader:   s3manager.NewUploader(sess),
		bucketName: bucketName,
		mockMode:   false,
	}
}

func (s *S3Client) UploadRecording(filePath, key string) (string, error) {
	if s.mockMode {
		// Mock mode - return a local file URL
		absPath, _ := filepath.Abs(filePath)
		mockURL := fmt.Sprintf("file://%s", absPath)
		log.Printf("📁 [MOCK] S3 upload: %s -> %s", filePath, mockURL)
		return mockURL, nil
	}

	// Real S3 upload
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	result, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return result.Location, nil
}
