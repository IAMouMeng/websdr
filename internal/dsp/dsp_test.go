package dsp

import (
	"math"
	"testing"
)

// TestResamplerRate verifies the fractional resampler produces the expected
// output rate continuously across block boundaries.
func TestResamplerRate(t *testing.T) {
	const inRate = 256000.0
	const outRate = 48000.0
	var rs Resampler

	in := make([]float32, 8533) // ~one 33ms block
	dst := make([]float32, 4096)

	total := 0
	const blocks = 30
	for b := 0; b < blocks; b++ {
		for i := range in {
			in[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(b*len(in)+i) / inRate))
		}
		total += rs.Resample(in, inRate, outRate, dst)
	}

	wantRate := float64(total) / (float64(blocks*len(in)) / inRate)
	if math.Abs(wantRate-outRate) > 100 {
		t.Fatalf("output rate %.1f Hz, want ~%.0f Hz", wantRate, outRate)
	}
}

// TestResamplerContinuity checks there is no large discontinuity at block
// boundaries when resampling a smooth tone.
func TestResamplerContinuity(t *testing.T) {
	const inRate = 250000.0
	const outRate = 48000.0
	var rs Resampler
	in := make([]float32, 5000)
	dst := make([]float32, 2048)

	var prev float32
	first := true
	phase := 0.0
	for b := 0; b < 10; b++ {
		for i := range in {
			in[i] = float32(math.Sin(phase))
			phase += 2 * math.Pi * 500 / inRate
		}
		n := rs.Resample(in, inRate, outRate, dst)
		for i := 0; i < n; i++ {
			if !first {
				if d := math.Abs(float64(dst[i] - prev)); d > 0.2 {
					t.Fatalf("discontinuity %.3f at block %d sample %d", d, b, i)
				}
			}
			prev = dst[i]
			first = false
		}
	}
}

func TestPowerSpectrumPeak(t *testing.T) {
	iq := make([]complex128, FFTSize)
	// Tone at +1/8 of the sample rate.
	for i := range iq {
		ph := 2 * math.Pi * float64(i) / 8
		iq[i] = complex(math.Cos(ph), math.Sin(ph))
	}
	win := HannWindow(FFTSize)
	fftBuf := make([]complex128, FFTSize)
	powBuf := make([]float64, FFTSize/2)
	out := make([]float32, FFTSize/2)
	PowerSpectrum(iq, 1, fftBuf, win, powBuf, out)

	peak, peakIdx := float32(-1e9), 0
	for i, v := range out {
		if v > peak {
			peak, peakIdx = v, i
		}
	}
	// A full-scale tone should land well above the noise floor (dB scale).
	if peak < -20 {
		t.Fatalf("peak too low: %.1f dB", peak)
	}
	// +Fs/8 should sit right of center (center = FFTSize/4 for the half view).
	if peakIdx <= FFTSize/4 {
		t.Fatalf("peak at bin %d, expected right of center %d", peakIdx, FFTSize/4)
	}
}
