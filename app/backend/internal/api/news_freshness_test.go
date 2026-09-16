package api

import (
	"testing"
	"time"

	"trading-agents/internal/dataflows"
)

func TestLatestNewsPublishedUsesArticleTimeNotFetchTime(t *testing.T) {
	items := []dataflows.NewsArticle{
		{Published: "Fri, 04 Sep 2026 13:05:00 +0000"},
		{Published: "Fri, 04 Sep 2026 14:05:00 +0000"},
	}
	want := time.Date(2026, 9, 4, 14, 5, 0, 0, time.UTC)
	if got := latestNewsPublished(items); !got.Equal(want) {
		t.Fatalf("data time=%s want=%s", got, want)
	}
}

func TestNewsFreshnessUsesPublicationTimeNotCacheTime(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	old := newsFreshness(now.Add(-48*time.Hour), now)
	if !old.Stale || old.DataTime != "2026-09-13T12:00:00Z" {
		t.Fatalf("old article must stay stale after a fresh fetch: %+v", old)
	}
	recent := newsFreshness(now.Add(-time.Hour), now)
	if recent.Stale || recent.RefreshedAt != now.Format(time.RFC3339) {
		t.Fatalf("recent article should be live with separate refresh time: %+v", recent)
	}
}
