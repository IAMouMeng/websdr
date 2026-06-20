package protocol

import (
	"math"

	"github.com/iamoumeng/websdr/internal/dsp"
)

// OQPSKConstellation mixes IQ to baseband, filters, and decimates to symbol
// rate for a scatter plot. Returns flat [I,Q,I,Q,...] normalized to ~±1.
func OQPSKConstellation(iq []complex128, sr, offsetHz, symRate float64, maxPoints int) []float64 {
	if len(iq) < 4096 || symRate <= 0 || maxPoints <= 0 {
		return nil
	}
	work := make([]complex128, len(iq))
	copy(work, iq)
	var phase float64
	dsp.MixDown(work, offsetHz, sr, &phase)

	n := len(work)
	if n > 65536 {
		work = work[:65536]
		n = len(work)
	}

	cutoff := symRate * 0.55
	if cutoff < 8000 {
		cutoff = 8000
	}
	if cutoff > sr*0.45 {
		cutoff = sr * 0.45
	}

	var filt dsp.FIR
	filt.ProcessComplex(work, cutoff, sr)

	sps := sr / symRate
	if sps < 2 {
		sps = 2
	}
	step := int(sps)
	if step < 2 {
		step = 2
	}

	// Estimate RMS for normalization.
	var power float64
	samples := 0
	for i := 0; i < n; i += step {
		re, im := real(work[i]), imag(work[i])
		power += re*re + im*im
		samples++
	}
	if samples == 0 {
		return nil
	}
	rms := math.Sqrt(power / float64(samples))
	if rms < 1e-9 {
		return nil
	}
	// Uniform gain — preserve cluster shape, do not normalize each point to unit circle.
	gain := 0.65 / rms

	pts := make([]float64, 0, maxPoints*2)
	for i := 0; i < n && len(pts) < maxPoints*2; i += step {
		pts = append(pts, real(work[i])*gain, imag(work[i])*gain)
	}
	return pts
}
