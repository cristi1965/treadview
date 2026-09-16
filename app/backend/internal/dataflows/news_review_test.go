package dataflows

import (
	"strings"
	"testing"
	"time"
)

func TestReviewTickerNewsFiltersClassifiesAndDeduplicates(t *testing.T) {
	articles := []NewsArticle{
		{Title: "Apple reports quarterly revenue", Link: "https://www.reuters.com/a?x=1", Published: "Mon, 07 Sep 2026 03:00:00 +0000", Source: "Reuters"},
		{Title: "Apple reports quarterly revenue", Link: "https://www.reuters.com/a?x=2", Published: "Mon, 07 Sep 2026 02:00:00 +0000", Source: "Reuters"},
		{Title: "Analyst says AAPL could rise", Link: "https://finance.yahoo.com/b", Published: "Mon, 07 Sep 2026 01:00:00 +0000", Source: "Yahoo Finance"},
		{Title: "Nvidia launches a chip", Link: "https://example.com/c", Published: "Mon, 07 Sep 2026 00:00:00 +0000", Source: "Other"},
		{Title: "Nvidia forecast puts it on a path to pass Apple", Link: "https://example.com/d", Published: "Mon, 07 Sep 2026 00:00:00 +0000", Source: "Other"},
		{Title: "Macro week ahead", Description: "Apple and Nvidia are among companies to watch", Link: "https://example.com/e", Published: "Mon, 07 Sep 2026 00:00:00 +0000", Source: "Other"},
	}
	review := ReviewTickerNews("AAPL", "Apple Inc.", articles)
	if review.InputCount != 6 || review.IncludedCount != 2 || review.DuplicatesRemoved != 1 || review.ExcludedLowRelevance != 3 {
		t.Fatalf("review counts=%+v", review)
	}
	if review.Items[0].SourceTier != "tier-1-primary-or-wire" || review.Items[0].Theme != "earnings-and-financials" || review.Items[1].ContentType != "opinion-or-forecast" {
		t.Fatalf("review classification=%+v", review.Items)
	}
	formatted := FormatReviewedNewsForEvidence(review, "Yahoo Finance RSS", "https://example.test/rss")
	if !strings.Contains(formatted, "Provider: Yahoo Finance RSS") {
		t.Fatalf("formatted review omitted provider: %s", formatted)
	}
	parsed, err := ParseNewsEvidenceReview(formatted)
	if err != nil || parsed.IncludedCount != 2 || len(parsed.Rules) == 0 {
		t.Fatalf("review round trip=%+v err=%v", parsed, err)
	}
}

func TestFilterNewsForTradeDateEnforcesPointInTimeWindow(t *testing.T) {
	articles := []NewsArticle{
		{Title: "within window", Published: "Fri, 04 Sep 2026 23:59:59 +0000"},
		{Title: "future day", Published: "Sat, 05 Sep 2026 00:00:00 +0000"},
		{Title: "old boundary", Published: "Sat, 29 Aug 2026 00:00:00 +0000"},
		{Title: "too old", Published: "Fri, 28 Aug 2026 23:59:59 +0000"},
		{Title: "undated", Published: "unknown"},
	}
	got := filterNewsForTradeDate(articles, "2026-09-04", 7*24*time.Hour)
	if len(got) != 2 || got[0].Title != "within window" || got[1].Title != "old boundary" {
		t.Fatalf("point-in-time filter returned %+v", got)
	}
	if got[0].Published != "2026-09-04T23:59:59Z" {
		t.Fatalf("published time was not normalized: %q", got[0].Published)
	}
	if got := filterNewsForTradeDate(articles, "invalid", 7*24*time.Hour); len(got) != 0 {
		t.Fatalf("invalid trade date should reject all news: %+v", got)
	}
}
