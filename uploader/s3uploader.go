package uploader

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func UploadToS3(s3AccessKey, s3SecretKey, bucketName, objectKey string, data []byte) (string, error) {
	// Create a session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-northeast-1"), // Change this to your AWS region
		Credentials: credentials.NewStaticCredentials(s3AccessKey, s3SecretKey, ""),
	})
	if err != nil {
		return "", err
	}

	// Create an S3 client
	svc := s3.New(sess)

	// Upload the file
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return "", err
	}

	// Generate the object URL
	objectURL := fmt.Sprintf("https://%s.s3-%s.amazonaws.com/%s", bucketName, *sess.Config.Region, objectKey)
	return objectURL, nil
}
