package protocol

import (
	"testing"
)

func TestDetectPeaksFindsCarrier(t *testing.T) {
	db := make([]float32, 1024)
	for i := range db {
		db[i] = -95
	}
	for i := 500; i < 530; i++ {
		db[i] = -55
	}
	peaks := DetectPeaks(db, 100_000_000, 2_048_000)
	if len(peaks) == 0 {
		t.Fatal("expected at least one peak")
	}
	if peaks[0].PowerDB < -60 {
		t.Fatalf("peak power too low: %v", peaks[0].PowerDB)
	}
}

func TestClassifyFM(t *testing.T) {
	var band Band
	for _, b := range Bands {
		if b.Name == "FM 广播" {
			band = b
			break
		}
	}
	sig := Classify(Peak{FreqHz: 99_700_000, PowerDB: -50, BwHz: 180_000}, band)
	if sig.Type != "broadcast" {
		t.Fatalf("type=%s want broadcast", sig.Type)
	}
}
