package dataflows

import "testing"

func TestParseAlpacaSnapshotsPreservesIEXProvenance(t *testing.T) {
	body := []byte(`{"AAPL":{"latestTrade":{"p":234.5,"t":"2026-09-11T14:31:02.123Z","x":"V"},"prevDailyBar":{"c":230}}}`)
	quotes, err := parseAlpacaSnapshots(body, "iex", "https://data.alpaca.markets/v2/stocks/snapshots?feed=iex")
	if err != nil {
		t.Fatal(err)
	}
	quote := quotes["AAPL"]
	if quote.Price != 234.5 || quote.PrevClose != 230 || quote.Feed != "iex" || quote.Exchange != "V" || quote.ObservedAt.IsZero() {
		t.Fatalf("unexpected quote: %+v", quote)
	}
}

func TestAlpacaClientWithoutCredentialsIsDisabled(t *testing.T) {
	client := NewAlpacaQuoteClient("", "")
	if client.Configured() {
		t.Fatal("client without credentials must be disabled")
	}
	quotes, err := client.GetQuotes([]string{"AAPL"})
	if err != nil || len(quotes) != 0 {
		t.Fatalf("disabled client must return immediately: quotes=%v err=%v", quotes, err)
	}
}
