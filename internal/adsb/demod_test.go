package adsb

import (
	"encoding/hex"
	"testing"
)

// synthFrame renders a 14-byte frame as a 2 Msps IQ waveform: an 8 µs preamble
// then 112 Manchester bits, with some quiet padding on each side.
func synthFrame(msg []byte) [][2]uint8 {
	hi := [2]uint8{255, 128}
	lo := [2]uint8{128, 128}
	var iq [][2]uint8
	for i := 0; i < 40; i++ {
		iq = append(iq, lo)
	}
	var pre [16]bool
	for _, p := range []int{0, 2, 7, 9} {
		pre[p] = true
	}
	for _, on := range pre {
		if on {
			iq = append(iq, hi)
		} else {
			iq = append(iq, lo)
		}
	}
	for k := 0; k < dataBits; k++ {
		bit := (msg[k>>3]>>uint(7-(k&7)))&1 == 1
		if bit {
			iq = append(iq, hi, lo)
		} else {
			iq = append(iq, lo, hi)
		}
	}
	for i := 0; i < 40; i++ {
		iq = append(iq, lo)
	}
	return iq
}

func TestDemodRoundTrip(t *testing.T) {
	want := "8D40621D58C382D690C8AC2863A7"
	wb, _ := hex.DecodeString(want)
	iq := synthFrame(wb)

	d := NewDemodulator()
	var got []string
	d.Process(iq, func(msg []byte) {
		got = append(got, hex.EncodeToString(msg))
	})
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1: %v", len(got), got)
	}
	if got[0] != "8d40621d58c382d690c8ac2863a7" {
		t.Errorf("frame = %s, want %s", got[0], want)
	}
}

func TestDemodSplitAcrossBlocks(t *testing.T) {
	wb, _ := hex.DecodeString("8D4840D6202CC371C32CE0576098")
	iq := synthFrame(wb)
	split := 60 // cut partway through the frame
	d := NewDemodulator()
	var got int
	emit := func(msg []byte) { got++ }
	d.Process(iq[:split], emit)
	d.Process(iq[split:], emit)
	if got != 1 {
		t.Fatalf("got %d frames across split, want 1", got)
	}
}
