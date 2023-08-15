package dbmanager

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"
)

type Site struct {
	gorm.Model
	Name string `gorm:"unique"`
	URL  string `gorm:"unique"`
	Rss  []Rss  `gorm:"foreignkey:SiteID"`
}

func (Site) TableName() string {
	return "sites"
}

type Rss struct {
	gorm.Model
	Title       string
	Link        string `gorm:"unique"`
	PublishedAt time.Time
	SiteID      uint
	Description string
	Imgurl      string
	Tag         string
}

func (Rss) TableName() string {
	return "rsses"
}

func SaveSiteAndFeedItemsToDB(db *gorm.DB, siteName, siteURL string, feed *gofeed.Feed, objectURLs []string) error {
	db.AutoMigrate(&Site{}, &Rss{})

	var site Site
	result := db.Where("url = ?", siteURL).First(&site)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		site = Site{
			Name: siteName,
			URL:  siteURL,
		}
		if err := db.Create(&site).Error; err != nil {
			return fmt.Errorf("failed to insert new site: %w", err)
		}
	}

	var rssItems []Rss
	linksSeen := make(map[string]bool) // リンクの一意性を保証するためのマップ

	for i, item := range feed.Items {
		publishedAt, err := time.Parse(time.RFC1123, item.Published)
		if err != nil {
			publishedAt = time.Now()
		}

		tags := ""
		for _, tag := range item.Categories {
			if tags != "" {
				tags += ", "
			}
			tags += tag
		}

		var imgurl string
		if i < len(objectURLs) {
			imgurl = objectURLs[i]
		} else {
			imgurl = ""
		}

		var rss Rss
		result := db.Where("link = ?", item.Link).First(&rss)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			if _, exists := linksSeen[item.Link]; !exists { // 重複チェック
				rss = Rss{
					Title:       item.Title,
					Link:        item.Link,
					PublishedAt: publishedAt,
					SiteID:      site.ID,
					Description: item.Description,
					Imgurl:      imgurl,
					Tag:         tags,
				}
				rssItems = append(rssItems, rss)
				linksSeen[item.Link] = true // マップにリンクを追加
			}
		}
	}

	numBatches := len(rssItems) / 500
	if len(rssItems)%500 != 0 {
		numBatches++
	}
	log.Printf("Number of batches: %d", numBatches)

	// デバッグ用：rssItemsの内容をログに出力
	log.Printf("DB保存されるアイテム: %d", len(rssItems))
	for _, rssItem := range rssItems {
		log.Printf("Title: %s, Link: %s", rssItem.Title, rssItem.Link)
	}

	if len(rssItems) > 0 {
		if err := db.CreateInBatches(rssItems, 500).Error; err != nil {
			return fmt.Errorf("failed to insert RSS items in batches: %w", err)
		}
		log.Printf("データベースに保存されたアイテム数: %d", len(rssItems)) // データベースに保存した後のログメッセージ
	}

	return nil

}
