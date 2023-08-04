package extractor

import (
	"strings"

	"golang.org/x/net/html"
)

func ExtractImageURL(content string) (string, error) {
	r := strings.NewReader(content)
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	var f func(*html.Node) string
	f = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, a := range n.Attr {
				if a.Key == "src" {
					return a.Val
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if url := f(c); url != "" {
				return url
			}
		}
		return ""
	}

	return f(doc), nil
}
