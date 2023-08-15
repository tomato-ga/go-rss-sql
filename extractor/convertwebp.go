package extractor

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // Required to identify gif images
	_ "image/jpeg" // This is required to decode jpeg images
	_ "image/png"  // This is required to decode png images
	"net/http"
	"strings"
	"time"

	webp "github.com/chai2010/webp"
)

func ConvertToWebP(url string) ([]byte, error) {
	// カスタムHTTPクライアントを作成
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	// 事前にHTTPステータスをチェック
	resp, err := client.Head(url)
	if err != nil {
		return nil, fmt.Errorf("HTTPヘッダの取得時のエラー: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("非200ステータスコードが返されました: %d", resp.StatusCode)
	}

	// サポートされていないフォーマットの早期検出
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "gif") {
		return nil, errors.New("GIF画像はサポートされていません")
	}

	// 画像を取得
	resp, err = client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("画像の取得時のエラー: %w", err)
	}
	defer resp.Body.Close()

	// 画像をデコード
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("画像のデコード時のエラー: %w", err)
	}

	// webpに変換
	quality := float32(85) // Or whatever quality level you want.
	webpData, err := webp.EncodeRGB(img, quality)
	if err != nil {
		return nil, fmt.Errorf("WebPへの変換エラー: %w", err)
	}

	return webpData, nil
}
