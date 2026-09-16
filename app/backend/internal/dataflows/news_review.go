package dataflows

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ReviewedNewsItem struct {
	Title         string `json:"title"`
	URL           string `json:"url"`
	PublishedAt   string `json:"published_at"`
	Source        string `json:"source"`
	Description   string `json:"description,omitempty"`
	SourceTier    string `json:"source_tier"`
	ContentType   string `json:"content_type"`
	Theme         string `json:"theme"`
	RelevanceRule string `json:"relevance_rule"`
}

type NewsEvidenceReview struct {
	Ticker               string             `json:"ticker"`
	CompanyName          string             `json:"company_name,omitempty"`
	InputCount           int                `json:"input_count"`
	IncludedCount        int                `json:"included_count"`
	ExcludedLowRelevance int                `json:"excluded_low_relevance"`
	DuplicatesRemoved    int                `json:"duplicates_removed"`
	Rules                []string           `json:"rules"`
	Items                []ReviewedNewsItem `json:"items"`
}

var newsTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func ReviewTickerNews(ticker, companyName string, articles []NewsArticle) NewsEvidenceReview {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	review := NewsEvidenceReview{
		Ticker: ticker, CompanyName: strings.TrimSpace(companyName), InputCount: len(articles),
		Rules: []string{
			"include only if the title contains the ticker token or identifies the company as a title subject; description-only and incidental list mentions are excluded",
			"deduplicate by normalized title and canonical URL without query or fragment",
			"source tier is a deterministic domain class; content type and theme are keyword classifications, not sentiment",
		},
		Items: []ReviewedNewsItem{},
	}
	companyTokens := meaningfulCompanyTokens(companyName)
	seenTitle := map[string]bool{}
	seenURL := map[string]bool{}
	for _, article := range articles {
		rule := directTitleRelevance(article.Title, strings.ToLower(ticker), companyTokens)
		if rule == "" {
			review.ExcludedLowRelevance++
			continue
		}
		titleKey := strings.Join(newsTokenPattern.FindAllString(strings.ToLower(article.Title), -1), " ")
		urlKey := canonicalNewsURL(article.Link)
		if (titleKey != "" && seenTitle[titleKey]) || (urlKey != "" && seenURL[urlKey]) {
			review.DuplicatesRemoved++
			continue
		}
		seenTitle[titleKey] = true
		seenURL[urlKey] = true
		publishedAt := strings.TrimSpace(article.Published)
		if parsed, ok := parseNewsPublishedTime(publishedAt); ok {
			publishedAt = parsed.UTC().Format(time.RFC3339)
		}
		review.Items = append(review.Items, ReviewedNewsItem{
			Title: article.Title, URL: article.Link, PublishedAt: publishedAt, Source: article.Source,
			Description: article.Description, SourceTier: newsSourceTier(article.Link),
			ContentType: newsContentType(article.Title + " " + article.Description),
			Theme:       newsTheme(article.Title + " " + article.Description), RelevanceRule: rule,
		})
	}
	sort.SliceStable(review.Items, func(i, j int) bool { return review.Items[i].PublishedAt > review.Items[j].PublishedAt })
	review.IncludedCount = len(review.Items)
	return review
}

func filterNewsForTradeDate(articles []NewsArticle, tradeDate string, maxAge time.Duration) []NewsArticle {
	tradeDay, err := time.Parse("2006-01-02", strings.TrimSpace(tradeDate))
	if err != nil {
		return nil
	}
	cutoff := tradeDay.Add(24 * time.Hour)
	earliest := cutoff.Add(-maxAge)
	filtered := make([]NewsArticle, 0, len(articles))
	for _, article := range articles {
		published, ok := parseNewsPublishedTime(article.Published)
		if !ok || !published.Before(cutoff) || published.Before(earliest) {
			continue
		}
		article.Published = published.UTC().Format(time.RFC3339)
		filtered = append(filtered, article)
	}
	return filtered
}

func parseNewsPublishedTime(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func directTitleRelevance(title, ticker string, companyTokens []string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	if containsNewsToken(lower, ticker) {
		return "ticker-title-token"
	}
	tokens := newsTokenPattern.FindAllString(lower, -1)
	for _, company := range companyTokens {
		index := -1
		for i, token := range tokens {
			if token == company {
				index = i
				break
			}
		}
		if index < 0 {
			continue
		}
		directPatterns := []string{
			company + " stock", company + " shares", company + "'s", company + " is ", company + " reports",
			company + " raises", company + " cuts", company + " launches", company + " sets", company + " warns",
			"for " + company + " stock", "lead " + company, "at " + company,
		}
		if newsContainsAny(lower, directPatterns...) {
			return "company-title-subject:" + company
		}
		// A leading company name is treated as the title subject unless the title
		// is visibly a comma-separated watch list.
		if index <= 1 && !strings.Contains(lower, ",") {
			return "company-title-subject:" + company
		}
	}
	return ""
}

func meaningfulCompanyTokens(name string) []string {
	generic := map[string]bool{"inc": true, "incorporated": true, "corp": true, "corporation": true, "company": true, "limited": true, "ltd": true, "holdings": true, "group": true, "the": true}
	seen := map[string]bool{}
	var out []string
	for _, token := range newsTokenPattern.FindAllString(strings.ToLower(name), -1) {
		if len(token) < 3 || generic[token] || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

func containsNewsToken(text, token string) bool {
	if token == "" {
		return false
	}
	for _, candidate := range newsTokenPattern.FindAllString(text, -1) {
		if candidate == token {
			return true
		}
	}
	return false
}

func canonicalNewsURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.ToLower(parsed.Host + strings.TrimSuffix(parsed.Path, "/"))
}

func newsSourceTier(raw string) string {
	host := ""
	if parsed, err := url.Parse(raw); err == nil {
		host = strings.ToLower(parsed.Hostname())
	}
	switch {
	case newsContainsAny(host, "sec.gov", "reuters.com", "apnews.com"):
		return "tier-1-primary-or-wire"
	case newsContainsAny(host, "bloomberg.com", "wsj.com", "ft.com", "cnbc.com", "marketwatch.com", "barrons.com", "finance.yahoo.com", "investopedia.com"):
		return "tier-2-established-media"
	default:
		return "tier-3-other"
	}
}

func newsContentType(text string) string {
	text = strings.ToLower(text)
	if newsContainsAny(text, "prediction", "forecast", "could", "may ", "might", "analyst", "opinion", "why ", "buy", "sell", "target price", "to watch") {
		return "opinion-or-forecast"
	}
	return "reported-fact"
}

func newsTheme(text string) string {
	text = strings.ToLower(text)
	switch {
	case newsContainsAny(text, "earnings", "revenue", "margin", "profit", "quarter", "guidance"):
		return "earnings-and-financials"
	case newsContainsAny(text, "ceo", "cfo", "chairman", "management", "board", "executive"):
		return "management-and-governance"
	case newsContainsAny(text, "antitrust", "lawsuit", "regulator", "sec ", "court", "tariff"):
		return "regulatory-and-legal"
	case newsContainsAny(text, "launch", "product", "iphone", "chip", "service", "factory", "supply"):
		return "products-and-operations"
	case newsContainsAny(text, "inflation", "federal reserve", "rates", "macro", "market"):
		return "macro-and-market"
	case newsContainsAny(text, "analyst", "rating", "target", "valuation"):
		return "analyst-and-valuation"
	default:
		return "other"
	}
}

func newsContainsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func FormatReviewedNewsForEvidence(review NewsEvidenceReview, provider, sourceURL string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Provider: %s\n", strings.TrimSpace(provider)))
	sb.WriteString(fmt.Sprintf("Source URL: %s\n", sourceURL))
	sb.WriteString(fmt.Sprintf("News review for %s: input=%d included=%d excluded_low_relevance=%d duplicates_removed=%d\n",
		review.Ticker, review.InputCount, review.IncludedCount, review.ExcludedLowRelevance, review.DuplicatesRemoved))
	for index, item := range review.Items {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", index+1, item.Title))
		sb.WriteString(fmt.Sprintf("   Published: %s\n   Source: %s | Tier: %s | Type: %s | Theme: %s | Relevance: %s\n   Link: %s\n",
			item.PublishedAt, item.Source, item.SourceTier, item.ContentType, item.Theme, item.RelevanceRule, item.URL))
	}
	raw, _ := json.Marshal(review)
	sb.WriteString("News Review JSON: ")
	sb.Write(raw)
	sb.WriteByte('\n')
	return sb.String()
}

func ParseNewsEvidenceReview(output string) (NewsEvidenceReview, error) {
	const marker = "News Review JSON: "
	index := strings.Index(output, marker)
	if index < 0 {
		return NewsEvidenceReview{}, fmt.Errorf("structured news review is missing")
	}
	raw := strings.TrimSpace(output[index+len(marker):])
	var review NewsEvidenceReview
	if err := json.Unmarshal([]byte(raw), &review); err != nil {
		return NewsEvidenceReview{}, err
	}
	return review, nil
}
