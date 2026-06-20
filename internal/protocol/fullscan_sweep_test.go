package protocol

import "testing"

func TestFullSweepBandsRange(t *testing.T) {
	bands := FullSweepBands()
	if len(bands) < 750 || len(bands) > 850 {
		t.Fatalf("expected ~800 sweep chunks with 2 MHz step, got %d", len(bands))
	}
	first := bands[0]
	last := bands[len(bands)-1]
	if first.CenterHz < fullScanStartHz {
		t.Fatalf("first center %d below min", first.CenterHz)
	}
	if bands[1].CenterHz-bands[0].CenterHz != fullScanStepHz {
		t.Fatalf("step=%d want %d", bands[1].CenterHz-bands[0].CenterHz, fullScanStepHz)
	}
	if last.CenterHz > FullScanMaxHz {
		t.Fatalf("last center %d above max", last.CenterHz)
	}
	// Each chunk spans the full sample rate.
	half := uint64(fullScanRateHz / 2)
	span := last.CenterHz + half - (first.CenterHz - half)
	wantMin := FullScanMaxHz - fullScanStartHz
	if span < wantMin/2 {
		t.Fatalf("coverage span %d too small", span)
	}
}

func TestSummarizeFullScanGrouped(t *testing.T) {
	rec := []BandRecord{
		{Band: Band{Name: "a", CenterHz: 100_000_000, RateHz: 2_048_000}, Signals: []Signal{{Type: "broadcast"}}},
		{Band: Band{Name: "b", CenterHz: 110_000_000, RateHz: 2_048_000}, Signals: []Signal{{Type: "broadcast"}}},
	}
	out := SummarizeFullScanGrouped(rec, 50_000_000)
	if len(out) < 2 {
		t.Fatalf("len=%d", len(out))
	}
}
