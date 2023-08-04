package extractor

import (
	"image"
	_ "image/jpeg" // This is required to decode jpeg images
	_ "image/png"  // This is required to decode png images
	"net/http"

	webp "github.com/chai2010/webp"
)

func ConvertToWebP(url string) ([]byte, error) {
	// Fetch the image
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the image
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	// Convert to webp
	quality := float32(75) // Or whatever quality level you want.
	webpData, err := webp.EncodeRGB(img, quality)
	if err != nil {
		return nil, err
	}

	return webpData, nil
}
