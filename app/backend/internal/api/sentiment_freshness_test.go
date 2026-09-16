package api

import (
	"testing"
	"time"
)

func TestSentimentSnapshotDataTimeUsesOldestGaugeObservation(t *testing.T) {
	snap := &sentimentSnapshot{
		Ts: "2026-09-15T10:00:00Z",
		US: sentimentPanel{Gauges: []sentimentGauge{{ID: "vix", AsOf: "2026-09-15"}, {ID: "tnx", AsOf: "2026-09-12"}}},
		CN: sentimentPanel{Gauges: []sentimentGauge{{ID: "fgi"}}},
	}
	got, ok := sentimentSnapshotDataTime(snap)
	want := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("oldest sentiment observation=%s ok=%v want=%s", got, ok, want)
	}
}
