package dataflows

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PlayerTake is one public post from StockTwits or an X/Twitter mention index.
type PlayerTake struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	Created   string `json:"created"`
	Sentiment string `json:"sentiment,omitempty"`
	Source    string `json:"source"`
	Link      string `json:"link,omitempty"`
}

type stockTwitsStream struct {
	Messages []struct {
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		User      struct {
			Username string `json:"username"`
		} `json:"user"`
		Entities struct {
			Sentiment *struct {
				Basic string `json:"basic"`
			} `json:"sentiment"`
		} `json:"entities"`
	} `json:"messages"`
}

// GetXPlayerTakes pulls public trader chatter: StockTwits stream + Google News hits that mention X/Twitter.
// This is not the official X API; posts are public indexes, for contrast — not orders.
func (c *NewsClient) GetXPlayerTakes(ticker string, limit int) ([]PlayerTake, error) {
	if limit <= 0 {
		limit = 16
	}
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	var out []PlayerTake

	st, err := c.fetchStockTwits(ticker, limit)
	if err == nil {
		out = append(out, st...)
	}
	xn, err := c.fetchXMentionsNews(ticker, 8)
	if err == nil {
		out = append(out, xn...)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (c *NewsClient) fetchStockTwits(ticker string, limit int) ([]PlayerTake, error) {
	u := fmt.Sprintf("https://api.stocktwits.com/api/2/streams/symbol/%s.json", url.PathEscape(ticker))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stocktwits %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var stream stockTwitsStream
	if err := json.Unmarshal(body, &stream); err != nil {
		return nil, err
	}
	var takes []PlayerTake
	for _, m := range stream.Messages {
		sent := ""
		if m.Entities.Sentiment != nil {
			sent = m.Entities.Sentiment.Basic
		}
		takes = append(takes, PlayerTake{
			Author:    m.User.Username,
			Body:      strings.TrimSpace(stripHTML(m.Body)),
			Created:   m.CreatedAt,
			Sentiment: sent,
			Source:    "StockTwits",
			Link:      fmt.Sprintf("https://stocktwits.com/symbol/%s", ticker),
		})
		if len(takes) >= limit {
			break
		}
	}
	return takes, nil
}

func (c *NewsClient) fetchXMentionsNews(ticker string, limit int) ([]PlayerTake, error) {
	q := fmt.Sprintf("%s (site:x.com OR site:twitter.com) (stock OR $%s)", ticker, ticker)
	u := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en", url.QueryEscape(q))
	articles, err := c.fetchRSS(u, "X/Twitter (news index)")
	if err != nil {
		return nil, err
	}
	var takes []PlayerTake
	for _, a := range articles {
		takes = append(takes, PlayerTake{
			Author:  a.Source,
			Body:    strings.TrimSpace(a.Title + " — " + stripHTML(a.Description)),
			Created: a.Published,
			Source:  "X mention (Google News)",
			Link:    a.Link,
		})
		if len(takes) >= limit {
			break
		}
	}
	return takes, nil
}

// FormatPlayerTakesForLLM renders street/X chatter in EN then ZH (---) for the results panel.
func FormatPlayerTakesForLLM(ticker string, takes []PlayerTake) string {
	var en, zh strings.Builder
	en.WriteString(fmt.Sprintf("## Street / X player takes for %s\n\n", ticker))
	zh.WriteString(fmt.Sprintf("## X / 股票玩家观点 · %s\n\n", ticker))
	en.WriteString("Sources: public StockTwits stream + Google News index of x.com/twitter mentions. Not an official X firehose. Chatter ≠ advice.\n\n")
	zh.WriteString("来源：StockTwits 公开流 + Google 新闻索引中的 x.com/twitter 提及。不是官方 X 接口。玩家发言≠投资建议。\n\n")
	if len(takes) == 0 {
		en.WriteString("No public player posts retrieved (network/index empty). Do **not** treat silence as consensus to dump a large position.\n")
		zh.WriteString("没有抓到公开玩家帖（网络或索引为空）。**不要**把「没抓到」当成该清仓的共识。\n")
		return en.String() + "\n---\n\n" + zh.String()
	}

	bull, bear, other := 0, 0, 0
	for _, t := range takes {
		switch strings.ToLower(t.Sentiment) {
		case "bullish":
			bull++
		case "bearish":
			bear++
		default:
			other++
		}
	}
	en.WriteString(fmt.Sprintf("Tagged posts: Bullish %d · Bearish %d · Unlabeled %d (n=%d)\n\n", bull, bear, other, len(takes)))
	zh.WriteString(fmt.Sprintf("标记统计：看多 %d · 看空 %d · 未标注 %d（共 %d 条）\n\n帖子正文多为英文原文，左侧对照只翻译框架与来源。\n\n", bull, bear, other, len(takes)))

	for i, t := range takes {
		body := t.Body
		if len(body) > 280 {
			body = body[:280] + "…"
		}
		sent := t.Sentiment
		if sent == "" {
			sent = "unlabeled"
		}
		sentZh := map[string]string{"bullish": "看多", "bearish": "看空", "unlabeled": "未标注"}[strings.ToLower(sent)]
		if sentZh == "" {
			sentZh = sent
		}
		srcZh := t.Source
		if strings.Contains(t.Source, "StockTwits") {
			srcZh = "StockTwits"
		} else if strings.Contains(t.Source, "X") || strings.Contains(t.Source, "Twitter") {
			srcZh = "X 提及（新闻索引）"
		}
		en.WriteString(fmt.Sprintf("%d. **@%s** (%s · %s)\n", i+1, emptyAuthor(t.Author), sent, t.Source))
		zh.WriteString(fmt.Sprintf("%d. **@%s**（%s · %s）\n", i+1, emptyAuthor(t.Author), sentZh, srcZh))
		if t.Created != "" {
			en.WriteString(fmt.Sprintf("   %s\n", t.Created))
			zh.WriteString(fmt.Sprintf("   %s\n", t.Created))
		}
		en.WriteString(fmt.Sprintf("   %s\n", body))
		zh.WriteString(fmt.Sprintf("   原文：%s\n", body))
		if t.Link != "" {
			en.WriteString(fmt.Sprintf("   %s\n", t.Link))
			zh.WriteString(fmt.Sprintf("   %s\n", t.Link))
		}
		en.WriteString("\n")
		zh.WriteString("\n")
	}
	return en.String() + "\n---\n\n" + zh.String()
}

func emptyAuthor(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}

func stripHTML(s string) string {
	out := s
	for {
		start := strings.Index(out, "<")
		end := strings.Index(out, ">")
		if start < 0 || end < 0 || end <= start {
			break
		}
		out = out[:start] + " " + out[end+1:]
	}
	return strings.Join(strings.Fields(out), " ")
}
