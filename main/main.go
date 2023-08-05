package main

import (
	"fmt"
	"go-rss-sql/dbmanager"
	"go-rss-sql/extractor"
	"go-rss-sql/rssList"
	"go-rss-sql/uploader"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

type FeedResult struct {
	Feed *gofeed.Feed
	Err  error
	Tags []string
}

func fetchFeed(url string, resultChan chan<- FeedResult, wg *sync.WaitGroup) {
	defer wg.Done()
	feed, err := gofeed.NewParser().ParseURL(url)
	tags := make([]string, len(feed.Items))
	for i, item := range feed.Items {
		tags[i] = strings.Join(item.Categories, ", ")
	}
	resultChan <- FeedResult{Feed: feed, Err: err, Tags: tags}
}

func main() {
	// ログファイルの設定
	logFile, err := os.OpenFile("/home/don/docker/go/go-rss-sql/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %s", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	err = godotenv.Load(".env")
	if err != nil {
		log.Printf("Error loading .env file: %s", err)
	}

	dbURL := "postgresql://postgres:example@192.168.0.25:5433/postgres?sslmode=disable"
	db, err := gorm.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Failed to connect to database: %s", err)
		return
	}
	defer db.Close()

	s3AccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	s3SecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	fmt.Printf("Access Key: %s, Secret Key: %s", s3AccessKey, s3SecretKey)

	start := time.Now()
	var wg sync.WaitGroup

	resultChan := make(chan FeedResult, len(rssList.Rss_urls))
	for _, url := range rssList.Rss_urls {
		wg.Add(1)
		go fetchFeed(url, resultChan, &wg)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Err != nil {
			log.Printf("Error fetching feed: %s", result.Err)
			continue
		}

		fmt.Println(result.Feed.Title)
		fmt.Println(result.Feed.FeedType, result.Feed.FeedVersion)

		var objectURLs []string

		for i, item := range result.Feed.Items {
			if item == nil {
				break
			}
			fmt.Println(item.Title)
			fmt.Println(item.Link)
			fmt.Println(item.PublishedParsed.Format(time.RFC3339))
			fmt.Println(result.Tags[i])

			imageURL, err := extractor.ExtractImageURL(item.Content)
			if err != nil || imageURL == "" {
				imageURL, err = extractor.ExtractImageURL(item.Description)
				if err != nil {
					log.Printf("画像URLの抽出に失敗しました: %s", err)
				}
			}

			if imageURL != "" {
				webpImage, err := extractor.ConvertToWebP(imageURL)
				if err != nil {
					log.Printf("WebPへの画像変換に失敗しました: %s", err)
				} else {
					objectKey := "photo/" + uuid.New().String() + ".webp"

					objectURL, err := uploader.UploadToS3(s3AccessKey, s3SecretKey, "rssphoto", objectKey, webpImage)
					if err != nil {
						log.Printf("S3への画像アップロードに失敗しました: %s", err)
					} else {
						fmt.Println("S3へ画像をアップロードしました: ", objectKey)
						objectURLs = append(objectURLs, objectURL)
					}
				}
			}
		}

		err = dbmanager.SaveSiteAndFeedItemsToDB(db, result.Feed.Title, result.Feed.Link, result.Feed, objectURLs)
		if err != nil {
			log.Printf("データベースへの保存に失敗しました: %s", err)
			continue
		}
	}

	elapsed := time.Since(start)
	fmt.Println(elapsed)
}
