package api

import "testing"

func TestMarketPayloadHeaderFreshnessFollowsPayloadAndSource(t *testing.T) {
	if !snapshotPayloadStale([]byte(`{"stale":true}`)) {
		t.Fatal("stale payload was treated as fresh")
	}
	if snapshotPayloadStale([]byte(`{"stale":false}`)) || snapshotPayloadStale([]byte(`not-json`)) {
		t.Fatal("fresh or malformed payload was treated as stale by the body helper")
	}
	if got := marketPayloadStaleReason("closed-nasdaq@2026-09-08", true); got != "market session closed; serving the last completed provider snapshot" {
		t.Fatalf("unexpected closed-market reason: %q", got)
	}
	if got := marketPayloadStaleReason("stale-snapshot:us-partial@2026-09-08T20:00:00Z", true); got != "market universe is only partially provider-timed; serving as a stale snapshot" {
		t.Fatalf("unexpected partial-market reason: %q", got)
	}
}
