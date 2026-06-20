package protocol

import "testing"

func TestBandReachable(t *testing.T) {
	if BandReachable(Band{CenterHz: 10_000_000}) {
		t.Fatal("10 MHz center is below RTL-SDR minimum")
	}
	if !BandReachable(Band{CenterHz: 28_300_000}) {
		t.Fatal("28 MHz 10m band should be reachable")
	}
}

func TestCWDedicatedBands(t *testing.T) {
	for _, b := range Bands {
		if b.Name == "10米 CW (28 MHz)" && !IsCWDedicatedBand(b) {
			t.Fatal("10m band should be CW dedicated")
		}
		if b.Name == "业余 2m" && IsCWDedicatedBand(b) {
			t.Fatal("2m mixed band should not be CW dedicated")
		}
	}
}
