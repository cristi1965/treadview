package gpupricing

import (
	"math"
	"testing"
)

func TestPublicReferenceIsHistoricalCompleteAndReadOnly(t *testing.T) {
	reference := PublicReference()
	if reference.Status != "historical" || reference.DataMode != "historical" || !reference.Stale || reference.Source != ReferenceSource || reference.VerifiedAt != ReferenceVerifiedAt {
		t.Fatalf("reference metadata=%+v", reference)
	}
	if reference.Count != 22 || len(reference.Items) != reference.Count || len(reference.Providers) != 4 {
		t.Fatalf("counts: items=%d count=%d providers=%d", len(reference.Items), reference.Count, len(reference.Providers))
	}
	providerCounts := map[string]int{}
	for _, item := range reference.Items {
		providerCounts[item.Provider]++
		if item.RawPrice <= 0 || item.PriceUSDPerGPUHour <= 0 || item.Currency != "USD" {
			t.Fatalf("invalid reference item=%+v", item)
		}
	}
	if providerCounts[ProviderRunPod] != 7 || providerCounts[ProviderModal] != 7 || providerCounts[ProviderLambda] != 8 || providerCounts[ProviderVast] != 0 {
		t.Fatalf("provider counts=%v", providerCounts)
	}
	if got := reference.Items[7]; got.RawUnit != "USD/GPU-second" || math.Abs(got.PriceUSDPerGPUHour-got.RawPrice*3600) > 1e-12 {
		t.Fatalf("Modal conversion=%+v", got)
	}
	if vast := reference.Providers[3]; vast.Provider != ProviderVast || vast.HasFixedReference || vast.Status != "market-variable" || vast.SourceURL == "" || vast.Note == "" {
		t.Fatalf("Vast reference=%+v", vast)
	}

	reference.Items[0].RawPrice = 999
	reference.Providers[0].Note = "mutated"
	again := PublicReference()
	if again.Items[0].RawPrice == 999 || again.Providers[0].Note == "mutated" {
		t.Fatal("public reference shared mutable package state")
	}
}
