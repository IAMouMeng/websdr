package protocol

import "testing"

func TestSnapAmateurSat(t *testing.T) {
	cases := []struct {
		inHz uint64
		want string
	}{
		{145_805_000, "ISS"},
		{436_790_000, "SO-50"},
		{145_840_000, ""},
		{433_920_000, ""},
	}
	for _, c := range cases {
		_, name := SnapAmateurSat(c.inHz)
		if name != c.want {
			t.Fatalf("SnapAmateurSat(%d) name=%q want %q", c.inHz, name, c.want)
		}
	}
}

func TestTryAmateurSatellite(t *testing.T) {
	peak := Peak{FreqHz: 436_792_000, PowerDB: -70, BwHz: 12_000}
	dr, ok := TryAmateurSatellite(peak)
	if !ok {
		t.Fatal("expected SO-50 detection")
	}
	if dr.Service != "业余卫星" {
		t.Fatalf("service=%q", dr.Service)
	}
	if dr.Label != "SO-50 436.795 MHz" {
		t.Fatalf("label=%q", dr.Label)
	}

	weak := Peak{FreqHz: 436_792_000, PowerDB: -90, BwHz: 12_000}
	if _, ok := TryAmateurSatellite(weak); ok {
		t.Fatal("weak signal should be rejected")
	}
}
