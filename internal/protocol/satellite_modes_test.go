package protocol

import "testing"

func TestSnapLRPTDownlink(t *testing.T) {
	if got := SnapLRPTDownlink(137_905_000); got != 137_900_000 {
		t.Fatalf("got %d", got)
	}
}

func TestTryHRPT(t *testing.T) {
	dr, ok := TryHRPT(Peak{FreqHz: 1_698_500_000, BwHz: 800_000, PowerDB: -70})
	if !ok || dr.Mod != "QPSK" {
		t.Fatalf("hrpt detect failed ok=%v mod=%s", ok, dr.Mod)
	}
}

func TestTryDSBFreq(t *testing.T) {
	if !isDSBFreq(137_770_000) {
		t.Fatal("expected DSB freq")
	}
}
