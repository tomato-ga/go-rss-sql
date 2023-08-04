package dbmanager

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/mmcdole/gofeed"
)

type Site struct {
	gorm.Model
	Name string `gorm:"unique"`
	URL  string `gorm:"unique"`
	Rss  []Rss
}

type Rss struct {
	gorm.Model
	Title       string
	Link        string `gorm:"unique"`
	PublishedAt time.Time
	SiteID      uint
	Description string
	ImgURL      string
}

func SaveSiteAndFeedItemsToDB(db *gorm.DB, siteName, siteURL string, feed *gofeed.Feed, objectURLs []string) error {
	db.AutoMigrate(&Site{}, &Rss{})

	var site Site
	if notFound := db.Where("url = ?", siteURL).First(&site).RecordNotFound(); notFound {
		// If the site does not exist yet, create a new record
		site = Site{
			Name: siteName,
			URL:  siteURL,
		}
		if err := db.Create(&site).Error; err != nil {
			return fmt.Errorf("failed to insert new site: %w", err)
		}
	}

	for i, item := range feed.Items {
		publishedAt, err := time.Parse(time.RFC1123, item.Published)
		if err != nil {
			publishedAt = time.Now()
		}

		rss := Rss{
			Title:       item.Title,
			Link:        item.Link,
			PublishedAt: publishedAt,
			SiteID:      site.ID,
			Description: item.Description,
			ImgURL:      objectURLs[i], // Set the actual image URL
		}
		if err := db.Create(&rss).Error; err != nil {
			return fmt.Errorf("failed to insert new RSS item: %w", err)
		}
	}

	return nil
}
