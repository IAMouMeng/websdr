package ais

import (
	"math"
	"math/cmplx"
	"testing"
)

// modulateFSK renders line bits as a complex baseband FSK signal: each bit is
// sps samples advancing the phase by ±2400 Hz (AIS GMSK deviation).
func modulateFSK(line []bool, sampleRate float64) []complex128 {
	sps := int(math.Round(sampleRate / SymbolRate))
	dev := 2400.0
	step := 2 * math.Pi * dev / sampleRate
	var out []complex128
	phase := 0.0
	// Lead-in idle so the discriminator/timing settle before the flags.
	for i := 0; i < 4*sps; i++ {
		phase += step
		out = append(out, cmplx.Rect(1, phase))
	}
	for _, b := range line {
		s := step
		if !b {
			s = -step
		}
		for i := 0; i < sps; i++ {
			phase += s
			out = append(out, cmplx.Rect(1, phase))
		}
	}
	return out
}

func TestGMSKRoundTrip(t *testing.T) {
	const rate = 48000.0
	body := payloadToBits("177KQJ5000G?tO`K>RA1wUbN0TKH", 0)
	line := hdlcEncode(body)
	iq := modulateFSK(line, rate)

	var got []*Report
	dm := NewChannelDemod(rate, func(b []bool) {
		if r := DecodePayload(b); r != nil {
			got = append(got, r)
		}
	})
	dm.Process(iq)

	if len(got) == 0 {
		t.Fatal("no frame demodulated")
	}
	if got[0].MMSI != 477553000 {
		t.Errorf("MMSI = %d, want 477553000", got[0].MMSI)
	}
	if !got[0].HasPos {
		t.Error("expected a position")
	}
}
