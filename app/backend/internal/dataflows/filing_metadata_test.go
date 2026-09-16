package dataflows

import (
	"strings"
	"testing"
)

func TestParseSECSubmissionFilingsMatchesSymbolFormAndPeriod(t *testing.T) {
	raw := []byte(`{"cik":"0000320193","tickers":["AAPL"],"filings":{"recent":{"accessionNumber":["0000320193-26-000079","wrong"],"filingDate":["2026-07-31","2026-08-01"],"reportDate":["2026-06-27","2026-06-27"],"form":["10-Q","8-K"]}}}`)
	got, err := parseSECSubmissionFilings(raw, "AAPL", 320193, []string{"2026-06-27"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	filing := got["2026-06-27"]
	if filing.FilingDate != "2026-07-31" || filing.Accession != "0000320193-26-000079" || !strings.Contains(filing.SourceURL, "000032019326000079") {
		t.Fatalf("unexpected filing: %+v", filing)
	}
	if _, err := parseSECSubmissionFilings(raw, "MSFT", 320193, []string{"2026-06-27"}, 2); err == nil {
		t.Fatal("symbol mismatch accepted")
	}
}

func TestParseNasdaqFilingsUsesNamespacedProviderReference(t *testing.T) {
	raw := []byte(`{"data":{"symbol":"AAPL","rows":[{"formType":"10-Q","filed":"07/31/2026","period":"06/27/2026","view":{"htmlLink":"https://app.quotemedia.com/data/downloadFiling?ref=320230447&type=HTML&symbol=AAPL&dateFiled=2026-07-31"}}]},"status":{"rCode":200}}`)
	got, err := parseNasdaqFilings(raw, "AAPL", []string{"2026-06-27"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	filing := got["2026-06-27"]
	if filing.FilingDate != "2026-07-31" || filing.Accession != "nasdaq-ref:320230447" || filing.Source != "nasdaq-sec-filings" {
		t.Fatalf("unexpected filing: %+v", filing)
	}
	if _, err := parseNasdaqFilings(raw, "NVDA", []string{"2026-06-27"}, 2); err == nil {
		t.Fatal("symbol mismatch accepted")
	}
	if _, err := parseNasdaqFilings(raw, "AAPL", []string{"2026-03-28"}, 2); err == nil {
		t.Fatal("period mismatch accepted")
	}
}
