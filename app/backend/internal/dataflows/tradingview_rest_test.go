package dataflows

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestTradingViewQuoteCarriesProviderObservationTime(t *testing.T) {
	want := time.Date(2026, 9, 8, 14, 30, 0, 0, time.UTC)
	client := &TVRestClient{httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"last_update_time"`) || !strings.Contains(string(body), `"update_mode"`) {
			t.Fatalf("provider time columns missing from request: %s", body)
		}
		payload := `{"totalCount":1,"data":[{"s":"NASDAQ:AAPL","d":[230,1,2,1000,231,228,229,228,` +
			fmtInt(want.Unix()) + `,"streaming"]}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
	})}}
	quotes, err := client.GetRealTimeQuotes([]string{"AAPL"})
	if err != nil || len(quotes) != 1 {
		t.Fatalf("quotes=%+v err=%v", quotes, err)
	}
	if !quotes[0].DataTime.Equal(want) || quotes[0].UpdateMode != "streaming" || quotes[0].SourceURL != tvScannerURL {
		t.Fatalf("provider provenance lost: %+v", quotes[0])
	}
}

func fmtInt(value int64) string {
	return fmt.Sprintf("%d", value)
}
