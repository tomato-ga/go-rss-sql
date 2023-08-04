package main

import (
	"fmt"
	"go-rss-sql/dbmanager"
	"go-rss-sql/extractor"
	"go-rss-sql/rssList"
	"go-rss-sql/uploader"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

// FeedResultは結果を格納するための構造体です
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
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("Error loading .env file")
	}

	dbURL := "postgresql://postgres:example@192.168.0.25:5433/postgres?sslmode=disable"
	db, err := gorm.Open("postgres", dbURL)

	if err != nil {
		fmt.Printf("Failed to connect to database: %s", err)
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
			fmt.Fprintln(os.Stderr, result.Err)
			continue
		}

		fmt.Println(result.Feed.Title)
		fmt.Println(result.Feed.FeedType, result.Feed.FeedVersion)

		var objectURLs []string // object URLを格納するためのスライスを新しく作成します

		for i, item := range result.Feed.Items {
			if item == nil {
				break
			}
			fmt.Println(item.Title)
			fmt.Println(item.Link)
			fmt.Println(item.PublishedParsed.Format(time.RFC3339))
			fmt.Println(result.Tags[i])

			// コンテンツから画像URLを抽出します
			imageURL, err := extractor.ExtractImageURL(item.Content)
			if err != nil || imageURL == "" {
				// コンテンツ内にimageURLが見つからなかった場合、またはエラーが発生した場合は、Descriptionから抽出を試みます
				imageURL, err = extractor.ExtractImageURL(item.Description)
				if err != nil {
					fmt.Println("画像URLの抽出に失敗しました: ", err)
				}
			}

			if imageURL != "" {
				webpImage, err := extractor.ConvertToWebP(imageURL)
				if err != nil {
					fmt.Println("WebPへの画像変換に失敗しました: ", err)
				} else {
					// それぞれの画像に対して一意のキーを生成します
					objectKey := "photo/" + uuid.New().String() + ".webp"

					// S3にWebP画像をアップロードします
					var objectURL string // objectURLをここで宣言します
					objectURL, err = uploader.UploadToS3(s3AccessKey, s3SecretKey, "rssphoto", objectKey, webpImage)
					if err != nil {
						fmt.Println("S3への画像アップロードに失敗しました: ", err)
					} else {
						fmt.Println("S3へ画像をアップロードしました: ", objectKey)
						objectURLs = append(objectURLs, objectURL) // object URLをスライスに追加します
					}
				}
			}
		}

		// サイトとフィードアイテムをデータベースに保存します
		err = dbmanager.SaveSiteAndFeedItemsToDB(db, result.Feed.Title, result.Feed.Link, result.Feed, objectURLs) // Site URLを引数として渡します
		if err != nil {
			fmt.Println("データベースへの保存に失敗しました: ", err)
			continue
		}
	}

	elapsed := time.Since(start)
	fmt.Println(elapsed)
}
