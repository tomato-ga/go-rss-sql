package main

import (
	"fmt"
	"go-rss-sql/rssList"
	"os"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// TODO 画像をS3にwebpで保存してURL取得する
// TODO DBに保存する

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

		for _, item := range result.Feed.Items {
			if item == nil {
				break
			}
			fmt.Println(item.Title)
			fmt.Println(item.Link)
			fmt.Println(item.PublishedParsed.Format(time.RFC3339))

			fmt.Printf("%+v\n", item)
		}
	}

	elapsed := time.Since(start)
	fmt.Println(elapsed)
}
