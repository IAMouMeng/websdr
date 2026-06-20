package ais

import (
	"math"
	"testing"
)

// Canonical AIVDM examples from the gpsd AIVDM/AIVDO decoding guide.

func TestDecodeType1(t *testing.T) {
	// !AIVDM,1,1,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0*5C
	bits := payloadToBits("177KQJ5000G?tO`K>RA1wUbN0TKH", 0)
	r := DecodePayload(bits)
	if r == nil {
		t.Fatal("decode returned nil")
	}
	if r.Type != 1 {
		t.Errorf("type = %d, want 1", r.Type)
	}
	if r.MMSI != 477553000 {
		t.Errorf("mmsi = %d, want 477553000", r.MMSI)
	}
	if r.NavStat != 5 {
		t.Errorf("navstat = %d, want 5", r.NavStat)
	}
	if r.SOG != 0 {
		t.Errorf("sog = %.1f, want 0", r.SOG)
	}
	if !r.HasPos {
		t.Fatal("expected a position")
	}
	if math.Abs(r.Lat-47.5828) > 0.01 || math.Abs(r.Lon-(-122.3458)) > 0.01 {
		t.Errorf("pos = %.4f, %.4f; want ~47.5828, ~-122.3458", r.Lat, r.Lon)
	}
}

func TestDecodeType18(t *testing.T) {
	// !AIVDM,1,1,,A,B5NJ;PP005l4ot5Isbl03wsUkP06,0*76  (Class B position report)
	bits := payloadToBits("B5NJ;PP005l4ot5Isbl03wsUkP06", 0)
	r := DecodePayload(bits)
	if r == nil || r.Type != 18 {
		t.Fatalf("type = %v, want 18", r)
	}
	if r.MMSI == 0 {
		t.Error("expected a non-zero MMSI")
	}
	if !r.HasPos {
		t.Error("expected a position for type 18")
	}
}
