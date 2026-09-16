package dataflows

import (
	"testing"
	"time"
)

func TestToEastmoneySecID(t *testing.T) {
	tests := map[string]string{
		"600519": "1.600519",
		"510300": "1.510300",
		"588000": "1.588000",
		"000001": "0.000001",
		"159915": "0.159915",
		"300750": "0.300750",
	}
	for code, want := range tests {
		if got := toEastmoneySecID(code); got != want {
			t.Errorf("toEastmoneySecID(%q)=%q want %q", code, got, want)
		}
	}
}

func TestParseCNQuotesRetainsProviderObservationTime(t *testing.T) {
	items, err := parseCNQuotes([]byte(`{"data":{"diff":[{"f12":"600519","f14":"贵州茅台","f2":1500.5,"f3":1.2,"f5":1234,"f20":2000000000000,"f124":1788852600}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Unix(1788852600, 0).UTC()
	if len(items) != 1 || items[0].Symbol != "600519" || !items[0].ObservedAt.Equal(want) {
		t.Fatalf("provider observation time lost: %+v", items)
	}
}
