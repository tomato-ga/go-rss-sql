package extractor

import (
	"errors"
	"image"
	_ "image/gif"  // Required to identify gif images
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
	img, format, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	// If the image format is GIF, return an error
	if format == "gif" {
		return nil, errors.New("GIF images are not supported")
	}

	// Convert to webp
	quality := float32(85) // Or whatever quality level you want.
	webpData, err := webp.EncodeRGB(img, quality)
	if err != nil {
		return nil, err
	}

	return webpData, nil
}
