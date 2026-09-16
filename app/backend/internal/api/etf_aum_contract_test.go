package api

import "testing"

func TestNormalizeETFAnalysisAUMRequiresExplicitUnit(t *testing.T) {
	missing := etfAnalysesFile{ETFs: []etfAnalysisEntry{{Sym: "VOO", AUM: 948673792}}}
	if err := normalizeETFAnalysisAUM(&missing); err == nil {
		t.Fatal("missing AUM unit must fail closed")
	}
	data := etfAnalysesFile{
		AUMUnit: "usd_thousands",
		Sectors: []etfSectorAnalysisEntry{{Sector: "broad", AUM: 4379761768}},
		ETFs:    []etfAnalysisEntry{{Sym: "VOO", AUM: 948673792}},
	}
	if err := normalizeETFAnalysisAUM(&data); err != nil {
		t.Fatal(err)
	}
	if data.AUMUnit != "usd" || data.ETFs[0].AUM != 948673792000 || data.Sectors[0].AUM != 4379761768000 {
		t.Fatalf("normalized=%+v", data)
	}
}
