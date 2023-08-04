package main

import (
	"fmt"
	"go-rss-sql/dbmanager"
	"go-rss-sql/extractor"
	"go-rss-sql/rssList"
	"go-rss-sql/uploader"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed"
)

// FeedResult is a structure to store the result
type FeedResult struct {
	Feed *gofeed.Feed
	Err  error
}

func fetchFeed(url string, resultChan chan<- FeedResult, wg *sync.WaitGroup) {
	defer wg.Done()
	feed, err := gofeed.NewParser().ParseURL(url)
	resultChan <- FeedResult{Feed: feed, Err: err}
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		fmt.Printf("Error loading .env file")
	}

	dbURL := "postgresql://postgres:example@192.168.0.25:5433/postgres"
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

		// Save site and feed items to database
		err = dbmanager.SaveSiteAndFeedItemsToDB(db, result.Feed.Title, "", result.Feed) // Please replace the second argument with the actual site URL
		if err != nil {
			fmt.Println("Failed to save to database: ", err)
			continue
		}

		for _, item := range result.Feed.Items {
			if item == nil {
				break
			}
			fmt.Println(item.Title)
			fmt.Println(item.Link)
			fmt.Println(item.PublishedParsed.Format(time.RFC3339))

			// extract the image URL from the content
			imageURL, err := extractor.ExtractImageURL(item.Content)
			if err != nil || imageURL == "" {
				// If imageURL is not found in the Content or an error occurred, try to extract from Description
				imageURL, err = extractor.ExtractImageURL(item.Description)
				if err != nil {
					fmt.Println("Failed to extract image URL: ", err)
				}
			}

			if imageURL != "" {
				webpImage, err := extractor.ConvertToWebP(imageURL)
				if err != nil {
					fmt.Println("Failed to convert image to WebP: ", err)
				} else {
					// Generate a unique key for each image
					objectKey := "photo/" + uuid.New().String() + ".webp"

					// Upload the WebP image to S3
					objectURL, err = uploader.UploadToS3(s3AccessKey, s3SecretKey, "rssphoto", objectKey, webpImage)
					if err != nil {
						fmt.Println("Failed to upload image to S3: ", err)
					} else {
						fmt.Println("Uploaded image to S3: ", objectKey)
					}
				}
			}
		}
	}

	elapsed := time.Since(start)
	fmt.Println(elapsed)
}
