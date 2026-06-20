package protocol

import (
	"math"
	"testing"
)

func TestGoertzelTone(t *testing.T) {
	sr := 200_000.0
	n := 8000
	x := make([]float32, n)
	for i := range x {
		x[i] = float32(math.Sin(2 * math.Pi * 2400 * float64(i) / sr))
	}
	r := toneRatio(x, sr, 2400)
	if r < 0.4 {
		t.Fatalf("tone ratio=%f want strong 2400Hz", r)
	}
}

func TestGoertzelOffTone(t *testing.T) {
	sr := 200_000.0
	n := 8000
	x := make([]float32, n)
	for i := range x {
		x[i] = float32(math.Sin(2 * math.Pi * 800 * float64(i) / sr))
	}
	r := toneRatio(x, sr, 2400)
	if r > 0.1 {
		t.Fatalf("tone ratio=%f want weak off-tone", r)
	}
}
