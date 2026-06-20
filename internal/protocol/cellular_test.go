package protocol

import "testing"

func TestInferGSM900ARFCN(t *testing.T) {
	info := InferCellular(935_732_000, 8000)
	if info.RAT != "GSM" {
		t.Fatalf("RAT=%s", info.RAT)
	}
	if info.Band != "GSM900" {
		t.Fatalf("Band=%s", info.Band)
	}
	if info.ARFCN == "—" {
		t.Fatal("expected ARFCN")
	}
}

func TestSnapGSMChannel(t *testing.T) {
	p := Peak{FreqHz: 935_732_000, PowerDB: -56, BwHz: 8000}
	s := snapCellFreq(p)
	if s.BwHz < 150_000 {
		t.Fatalf("bw=%f want ~200k", s.BwHz)
	}
}
