package protocol

import "testing"

func TestSnapAPTDownlink(t *testing.T) {
	cases := []struct {
		in, want uint64
	}{
		{137_115_000, 137_100_000},
		{137_139_000, 137_139_000}, // too far to snap (39 kHz)
		{137_158_000, 137_158_000},
		{137_100_000, 137_100_000},
		{137_905_000, 137_912_500},
		{137_600_000, 137_620_000},
		{145_825_000, 145_825_000},
	}
	for _, c := range cases {
		if got := SnapAPTDownlink(c.in); got != c.want {
			t.Fatalf("SnapAPTDownlink(%d) = %d want %d", c.in, got, c.want)
		}
	}
}

func TestIsNearAPTDownlink(t *testing.T) {
	if IsNearAPTDownlink(137_139_000) {
		t.Fatal("137.139 MHz should not be near an APT downlink")
	}
	if !IsNearAPTDownlink(137_110_000) {
		t.Fatal("137.110 MHz should be near NOAA 19")
	}
}

func TestCollapseAPTPeaks(t *testing.T) {
	far := []Peak{
		{FreqHz: 137_158_000, PowerDB: -55, BwHz: 40_000},
		{FreqHz: 137_139_000, PowerDB: -51, BwHz: 40_000},
	}
	out := CollapseAPTPeaks(far)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2 passthrough peaks", len(out))
	}
	for _, p := range out {
		if p.FreqHz == 137_100_000 {
			t.Fatalf("off-channel peaks must not snap to 137.1 MHz: %+v", out)
		}
	}

	near := []Peak{
		{FreqHz: 137_112_000, PowerDB: -55, BwHz: 40_000},
		{FreqHz: 137_118_000, PowerDB: -51, BwHz: 40_000},
	}
	out = CollapseAPTPeaks(near)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1 merged APT peak", len(out))
	}
	if out[0].FreqHz != 137_100_000 {
		t.Fatalf("freq=%d want 137100000", out[0].FreqHz)
	}
}
