package dataflows

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewsArticle represents a single news item.
type NewsArticle struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Published   string `json:"published"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// NewsClient fetches news from Yahoo Finance RSS and Google News.
type NewsClient struct {
	httpClient *http.Client
}

// NewNewsClient creates a news fetcher.
func NewNewsClient() *NewsClient {
	return &NewsClient{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// GetTickerNews fetches news for a specific ticker.
func (c *NewsClient) GetTickerNews(ticker string, limit int) ([]NewsArticle, error) {
	// Yahoo Finance RSS feed
	u := fmt.Sprintf("https://feeds.finance.yahoo.com/rss/2.0/headline?s=%s&region=US&lang=en-US",
		url.QueryEscape(ticker))

	articles, err := c.fetchRSS(u, "Yahoo Finance")
	if err != nil {
		// Fallback: return empty with no error
		return []NewsArticle{}, nil
	}

	if len(articles) > limit {
		articles = articles[:limit]
	}
	return articles, nil
}

// GetGlobalNews fetches global/macro news using search queries.
func (c *NewsClient) GetGlobalNews(queries []string, limit int) ([]NewsArticle, error) {
	var allArticles []NewsArticle

	for _, query := range queries {
		u := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en",
			url.QueryEscape(query))

		articles, err := c.fetchRSS(u, "Google News")
		if err != nil {
			continue
		}
		allArticles = append(allArticles, articles...)
	}

	if len(allArticles) > limit {
		allArticles = allArticles[:limit]
	}
	return allArticles, nil
}

// FormatNewsForLLM formats news articles into a string for LLM consumption.
func FormatNewsForLLM(articles []NewsArticle, header string) string {
	if len(articles) == 0 {
		return fmt.Sprintf("%s: No news articles found.", header)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%d articles):\n\n", header, len(articles)))
	for i, a := range articles {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, a.Title))
		if a.Published != "" {
			sb.WriteString(fmt.Sprintf("   Published: %s\n", a.Published))
		}
		if a.Source != "" {
			sb.WriteString(fmt.Sprintf("   Source: %s\n", a.Source))
		}
		if a.Description != "" {
			desc := a.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", desc))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// RSS XML structures
type rssRoot struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Source      string `xml:"source"`
}

func (c *NewsClient) fetchRSS(rawURL, source string) ([]NewsArticle, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS %s returned %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss rssRoot
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("parse RSS: %w", err)
	}

	var articles []NewsArticle
	for _, item := range rss.Channel.Items {
		a := NewsArticle{
			Title:       item.Title,
			Link:        item.Link,
			Published:   item.PubDate,
			Source:      source,
			Description: item.Description,
		}
		if item.Source != "" {
			a.Source = item.Source
		}
		articles = append(articles, a)
	}

	return articles, nil
}
