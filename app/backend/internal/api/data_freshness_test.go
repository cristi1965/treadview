package api

import (
	"strings"
	"testing"
	"time"
)

func TestStaleIfOlderRejectsFutureDataTime(t *testing.T) {
	future := time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339)
	stale, reason := staleIfOlder(future, time.Hour)
	if !stale {
		t.Fatal("future data time must be stale")
	}
	if !strings.Contains(reason, "in the future") {
		t.Fatalf("reason=%q", reason)
	}
}

func TestStaleIfOlderAcceptsRecentDataTime(t *testing.T) {
	recent := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	stale, reason := staleIfOlder(recent, time.Hour)
	if stale {
		t.Fatalf("recent data unexpectedly stale: %s", reason)
	}
}

func TestParseLooseDataTimeUsesNewYorkDaylightSavingRules(t *testing.T) {
	summer, ok := parseLooseDataTime("2026-09-14 10:00 ET")
	if !ok || summer.UTC().Format(time.RFC3339) != "2026-09-14T14:00:00Z" {
		t.Fatalf("summer ET=%s ok=%v", summer.UTC().Format(time.RFC3339), ok)
	}
	winter, ok := parseLooseDataTime("2026-01-14 10:00 ET")
	if !ok || winter.UTC().Format(time.RFC3339) != "2026-01-14T15:00:00Z" {
		t.Fatalf("winter ET=%s ok=%v", winter.UTC().Format(time.RFC3339), ok)
	}
}
