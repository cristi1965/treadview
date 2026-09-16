package market

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRefreshTiersKeepBroadSnapshotSeparateFromHotCadence(t *testing.T) {
	if liveRefreshInterval >= broadRefreshInterval {
		t.Fatalf("hot interval %s must be shorter than broad interval %s", liveRefreshInterval, broadRefreshInterval)
	}
	if broadRefreshInterval != 15*time.Minute {
		t.Fatalf("broad refresh interval=%s", broadRefreshInterval)
	}
	raw, err := os.ReadFile("live.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if !strings.Contains(source, "RefreshUSQuotes(0)") || !strings.Contains(source, "RefreshCNQuotes(0)") {
		t.Fatal("broad refresh must attempt both market universes")
	}
}
