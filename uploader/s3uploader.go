package uploader

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func UploadToS3(s3AccessKey, s3SecretKey, bucketName, objectKey string, data []byte) (string, error) {
	// ログファイルを開く
	logFile, err := os.OpenFile("./uploader.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("ログファイルのオープンに失敗: %v", err)
	}
	defer logFile.Close()

	// ロガーを作成
	logger := log.New(logFile, "", log.LstdFlags)

	logger.Println("S3へのアップロードを開始")

	// セッションを作成
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-northeast-1"), // ご自身のAWSリージョンに合わせて変更してください
		Credentials: credentials.NewStaticCredentials(s3AccessKey, s3SecretKey, ""),
	})
	if err != nil {
		logger.Println("AWSセッションの作成エラー:", err)
		return "", err
	}

	// S3クライアントを作成
	svc := s3.New(sess)

	// ファイルをアップロード
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		logger.Println("S3へのアップロードエラー:", err)
		return "", err
	}
	logger.Println("S3へのアップロード成功")

	// CloudFrontのURLを生成（直接のS3 URLの代わりに）
	objectURL := fmt.Sprintf("https://dr3jjw5otuz25.cloudfront.net/%s", objectKey)
	return objectURL, nil
}
