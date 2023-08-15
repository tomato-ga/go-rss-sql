package main

import (
	"errors"
	"go-rss-sql/dbmanager"
	"go-rss-sql/extractor"
	"go-rss-sql/rssList"
	"go-rss-sql/uploader"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

	urls := rssList.Rss_urls // すべてのURLを取得

	// ログファイルの設定
	logFile, err := os.OpenFile("./app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("ログファイルを開くのに失敗しました: %s", err)
	}
	defer logFile.Close()
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	err = godotenv.Load(".env") // /home/don/docker/go/go-rss-sql/main/
	if err != nil {
		log.Printf(".envファイルの読み込みにエラーが発生しました: %s", err)
	}

	dbURL := "postgresql://postgres:example@192.168.0.25:5433/postgres?sslmode=disable"
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Printf("データベースへの接続に失敗しました: %s", err)
		return
	}

	s3AccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	s3SecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	log.Printf("アクセスキー: %s, シークレットキー: %s", s3AccessKey, s3SecretKey)

	start := time.Now()
	var wg sync.WaitGroup

	resultChan := make(chan FeedResult, len(urls))
	for _, url := range urls {
		wg.Add(1)
		go fetchFeed(url, resultChan, &wg)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for feedResult := range resultChan {
		if feedResult.Err != nil {
			log.Printf("フィードの取得にエラーが発生しました: %s", feedResult.Err)
			continue
		}

		log.Printf("フィードのタイトル: %s", feedResult.Feed.Title)
		log.Printf("フィードタイプ: %s, バージョン: %s", feedResult.Feed.FeedType, feedResult.Feed.FeedVersion)

		var objectURLs []string

		for i, item := range feedResult.Feed.Items {
			var existingRss dbmanager.Rss
			queryResult := db.Where("link = ?", item.Link).First(&existingRss)
			if errors.Is(queryResult.Error, gorm.ErrRecordNotFound) {
				log.Printf("RSSアイテム '%s' はデータベースに存在しないため、アップロードおよび保存を行います。", item.Link)
				// 公開日がnilかどうかをチェックして、nilの場合はデフォルトの値を使用する
				var publishedDate string
				if item.PublishedParsed != nil {
					publishedDate = item.PublishedParsed.Format(time.RFC3339)
				} else {
					publishedDate = "不明"
				}
				log.Printf("アイテムタイトル: %s, リンク: %s, 公開日: %s, タグ: %s", item.Title, item.Link, publishedDate, feedResult.Tags[i])

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
			} else if queryResult.Error == nil {
				log.Printf("RSSアイテム '%s' は既にデータベースに存在します。アップロードおよび保存をスキップします。", item.Link)
			} else {
				log.Printf("データベースクエリ中にエラーが発生しました: %s", queryResult.Error)
			}
		}
		if len(objectURLs) > 0 {
			err = dbmanager.SaveSiteAndFeedItemsToDB(db, feedResult.Feed.Title, feedResult.Feed.Link, feedResult.Feed, objectURLs)
			if err != nil {
				log.Printf("データベースへの保存に失敗しました: %s", err)
				continue
			}
		}
	}

	elapsed := time.Since(start)
	log.Printf("所要時間: %s", elapsed)
}
