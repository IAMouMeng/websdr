package protocol

import "testing"

func TestSignalsForBand(t *testing.T) {
	band := Bands[0]
	all := []Signal{
		{ID: "1", Type: "cw", FreqHz: 28_300_000},
		{ID: "2", Type: "broadcast", FreqHz: 98_000_000},
	}
	got := SignalsForBand(all, band)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("got %+v", got)
	}
}

func TestSummarizeBandEmpty(t *testing.T) {
	band := Band{Name: "FM 广播", CenterHz: 98_000_000}
	sum := SummarizeBand(BandRecord{Band: band})
	if sum.SignalCount != 0 || sum.Summary == "" {
		t.Fatalf("sum=%+v", sum)
	}
}

func TestFullScanKeepSignal(t *testing.T) {
	if !FullScanKeepSignal(Signal{Type: "broadcast"}) {
		t.Fatal("broadcast should always show")
	}
	if FullScanKeepSignal(Signal{Type: "ism"}) {
		t.Fatal("undecoded ism should be hidden")
	}
	if !FullScanKeepSignal(Signal{Type: "ism", Decoded: true}) {
		t.Fatal("decoded ism should show")
	}
	if FullScanKeepSignal(Signal{Type: "cw"}) {
		t.Fatal("undecoded cw should be hidden")
	}
	if !FullScanKeepSignal(Signal{Type: "cw", Decoded: true}) {
		t.Fatal("decoded cw should show")
	}
}

func TestAggregateSignalsFromRecords(t *testing.T) {
	records := []BandRecord{
		{Signals: []Signal{{ID: "a", FreqHz: 98_000_000}}},
		{Signals: []Signal{{ID: "a", FreqHz: 98_000_000}, {ID: "b", FreqHz: 100_000_000}}},
	}
	got := AggregateSignalsFromRecords(records)
	if len(got) != 2 {
		t.Fatalf("want 2 unique signals, got %d", len(got))
	}
}
