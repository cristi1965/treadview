package dataflows

import (
	"strings"
	"testing"
)

func TestFormatPlayerTakesForLLMEmpty(t *testing.T) {
	s := FormatPlayerTakesForLLM("CRWV", nil)
	if !strings.Contains(s, "CRWV") || !strings.Contains(s, "No public player posts") || !strings.Contains(s, "没有抓到") {
		t.Fatalf("empty format: %s", s)
	}
	if !strings.Contains(s, "\n---\n") {
		t.Fatal("expected bilingual divider")
	}
}

func TestFormatPlayerTakesForLLMCounts(t *testing.T) {
	s := FormatPlayerTakesForLLM("CRWV", []PlayerTake{
		{Author: "alice", Body: "still holding", Sentiment: "Bullish", Source: "StockTwits"},
		{Author: "bob", Body: "trim a bit", Sentiment: "Bearish", Source: "StockTwits"},
		{Author: "carol", Body: "watching", Source: "X mention (Google News)"},
	})
	for _, part := range []string{"Bullish 1", "Bearish 1", "Unlabeled 1", "@alice", "still holding", "看多 1", "看空 1", "原文：still holding"} {
		if !strings.Contains(s, part) {
			t.Fatalf("missing %q in %s", part, s)
		}
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML("hello <b>world</b> & more")
	if got != "hello world & more" {
		t.Fatalf("got %q", got)
	}
}
