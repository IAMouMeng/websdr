package adsb

import (
	"encoding/hex"
	"math"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestCRCValid(t *testing.T) {
	for _, s := range []string{
		"8D4840D6202CC371C32CE0576098",
		"8D40621D58C382D690C8AC2863A7",
		"8D40621D58C386435CC412692AD6",
		"8D485020994409940838175B284F",
	} {
		if got := crc(mustHex(t, s)); got != 0 {
			t.Errorf("crc(%s) = %06X, want 0", s, got)
		}
	}
}

func TestDecodeCallsign(t *testing.T) {
	m := Decode(mustHex(t, "8D4840D6202CC371C32CE0576098"))
	if m == nil {
		t.Fatal("decode returned nil")
	}
	if m.ICAO != 0x4840D6 {
		t.Errorf("ICAO = %06X, want 4840D6", m.ICAO)
	}
	if m.Callsign != "KLM1023" {
		t.Errorf("callsign = %q, want KLM1023", m.Callsign)
	}
}

func TestDecodePositionGlobal(t *testing.T) {
	even := Decode(mustHex(t, "8D40621D58C382D690C8AC2863A7"))
	odd := Decode(mustHex(t, "8D40621D58C386435CC412692AD6"))
	if even == nil || odd == nil {
		t.Fatal("decode returned nil")
	}
	if !even.HasAltitude || even.Altitude != 38000 {
		t.Errorf("altitude = %d (has=%v), want 38000", even.Altitude, even.HasAltitude)
	}
	if even.CPROdd || !odd.CPROdd {
		t.Fatalf("odd/even flags wrong: even.odd=%v odd.odd=%v", even.CPROdd, odd.CPROdd)
	}
	lat, lon, ok := cprGlobal(even.LatCPR, even.LonCPR, odd.LatCPR, odd.LonCPR, true)
	if !ok {
		t.Fatal("cprGlobal failed")
	}
	if math.Abs(lat-52.2572) > 1e-3 || math.Abs(lon-3.91937) > 1e-3 {
		t.Errorf("pos = %.5f, %.5f; want 52.2572, 3.91937", lat, lon)
	}
}

func TestDecodeVelocity(t *testing.T) {
	m := Decode(mustHex(t, "8D485020994409940838175B284F"))
	if m == nil || !m.HasVelocity {
		t.Fatal("no velocity decoded")
	}
	if math.Abs(m.GroundSpeed-159) > 0.5 {
		t.Errorf("speed = %.1f, want ~159", m.GroundSpeed)
	}
	if math.Abs(m.Heading-182.88) > 0.1 {
		t.Errorf("heading = %.2f, want ~182.88", m.Heading)
	}
	if m.VertRate != -832 {
		t.Errorf("vert rate = %d, want -832", m.VertRate)
	}
}
