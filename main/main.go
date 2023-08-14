package main

import (
	"go-rss-sql/dbmanager"
	"go-rss-sql/extractor"
	"go-rss-sql/rssList"
	"go-rss-sql/uploader"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
	customClient := &http.Client{
		Timeout: 4 * time.Second,
	}
	fp := gofeed.NewParser()
	fp.Client = customClient
	feed, err := fp.ParseURL(url)
	if err != nil {
		resultChan <- FeedResult{Err: err}
		return
	}

	tags := make([]string, len(feed.Items))
	for i, item := range feed.Items {
		tags[i] = strings.Join(item.Categories, ", ")
	}
	resultChan <- FeedResult{Feed: feed, Err: err, Tags: tags}
}

func main() {

	if len(os.Args) < 2 {
		log.Fatalf("セグメントインデックスを指定してください")
	}

	segmentIndex, err := strconv.Atoi(os.Args[1])
	if err != nil || segmentIndex < 0 || segmentIndex > 2 {
		log.Fatalf("セグメントインデックスは0, 1, 2のいずれかでなければなりません")
	}

	urls := rssList.GetSegment(segmentIndex, 3)

	// ログファイルの設定
	logFile, err := os.OpenFile("./app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("ログファイルを開くのに失敗しました: %s", err)
	}
	defer logFile.Close()
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	err = godotenv.Load("/home/don/docker/go/go-rss-sql/main/.env")
	if err != nil {
		log.Printf(".envファイルの読み込みにエラーが発生しました: %s", err)
	}

	dbURL := "postgresql://postgres:dondonbex@54.64.237.55:5432/postgres?sslmode=disable"
	db, err := gorm.Open("postgres", dbURL)
	if err != nil {
		log.Printf("データベースへの接続に失敗しました: %s", err)
		return
	}
	defer db.Close()

	s3AccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	s3SecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	log.Printf("アクセスキー: %s, シークレットキー: %s", s3AccessKey, s3SecretKey)

	start := time.Now()
	var wg sync.WaitGroup

	resultChan := make(chan FeedResult, len(urls)) // `rssList.Rss_urls` を `urls` に変更
	for _, url := range urls {                     // `rssList.Rss_urls` を `urls` に変更
		wg.Add(1)
		go fetchFeed(url, resultChan, &wg)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.Err != nil {
			log.Printf("フィードの取得にエラーが発生しました: %s", result.Err)
			continue
		}

		log.Printf("フィードのタイトル: %s", result.Feed.Title)
		log.Printf("フィードタイプ: %s, バージョン: %s", result.Feed.FeedType, result.Feed.FeedVersion)

		var objectURLs []string

		// TODO データベースに存在するかどうか確認。もう一度やり直し。URLを減らす
		for i, item := range result.Feed.Items {
			var existingRss dbmanager.Rss
			if !db.Where("link = ?", item.Link).First(&existingRss).RecordNotFound() {
				log.Printf("RSSアイテム '%s' は既にデータベースに存在します。アップロードおよび保存をスキップします。", item.Link)
				continue
			}

			log.Printf("アイテムタイトル: %s, リンク: %s, 公開日: %s, タグ: %s", item.Title, item.Link, item.PublishedParsed.Format(time.RFC3339), result.Tags[i])

			imageURL, err := extractor.ExtractImageURL(item.Content)
			if err != nil || imageURL == "" {
				imageURL, err = extractor.ExtractImageURL(item.Description)
				if err != nil {
					log.Printf("画像URLの抽出に失敗しました: %s", err)
					continue
				}
			}

			if imageURL != "" {
				webpImage, err := extractor.ConvertToWebP(imageURL)
				if err != nil {
					log.Printf("WebPへの画像変換に失敗しました: %s", err)
					continue
				}

				objectKey := "photo/" + uuid.New().String() + ".webp"
				objectURL, err := uploader.UploadToS3(s3AccessKey, s3SecretKey, "erorice", objectKey, webpImage)
				if err != nil {
					log.Printf("S3への画像アップロードに失敗しました: %s", err)
					continue
				} else {
					log.Printf("S3へ画像をアップロードしました: %s", objectKey)
					objectURLs = append(objectURLs, objectURL)
				}
			}
		}

		if len(objectURLs) > 0 {
			err = dbmanager.SaveSiteAndFeedItemsToDB(db, result.Feed.Title, result.Feed.Link, result.Feed, objectURLs)
			if err != nil {
				log.Printf("データベースへの保存に失敗しました: %s", err)
				continue
			}
		}
	}

	elapsed := time.Since(start)
	log.Printf("所要時間: %s", elapsed)
}
