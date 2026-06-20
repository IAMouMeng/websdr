package dsp

import (
	"math"
	"math/cmplx"
)

// HannWindow returns a Hann window of length n.
func HannWindow(n int) []float64 {
	w := make([]float64, n)
	for i := 0; i < n; i++ {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// PowerSpectrum computes an averaged power spectrum over up to `segments`
// overlapping FFTSize windows taken from iq, and writes the result to dbOut
// (length FFTSize/2) as raw (uncalibrated) dB. Averaging several segments
// lowers the variance, giving a far less noisy waterfall. The caller decides
// how to map the dB range onto the display.
//
// fftBuf (len FFTSize), window (len FFTSize, Hann) and powBuf (len FFTSize/2)
// are caller-owned scratch buffers reused across frames.
func PowerSpectrum(iq []complex128, segments int, fftBuf []complex128, window []float64, powBuf []float64, dbOut []float32) {
	half := FFTSize / 2
	for i := range powBuf {
		powBuf[i] = 0
	}

	// Stride the available samples so the segments span the whole block.
	maxStart := len(iq) - FFTSize
	if maxStart < 0 {
		maxStart = 0
	}
	if segments < 1 {
		segments = 1
	}
	used := 0
	for s := 0; s < segments; s++ {
		start := 0
		if segments > 1 && maxStart > 0 {
			start = maxStart * s / (segments - 1)
		}
		end := start + FFTSize
		if end > len(iq) {
			break
		}
		for i := 0; i < FFTSize; i++ {
			fftBuf[i] = iq[start+i] * complex(window[i], 0)
		}
		fftInPlace(fftBuf)
		// Map the `half` output bins across the WHOLE sampled band (DC at the
		// center, ±fs/2 at the edges), so the spectrum aligns with the
		// frequency axis the frontend draws. The full FFT has FFTSize bins, so
		// each output bin averages the two fft-shifted bins that fall in it.
		for i := 0; i < half; i++ {
			b0 := (2*i + half) % FFTSize
			b1 := (2*i + 1 + half) % FFTSize
			powBuf[i] += (cmplx.Abs(fftBuf[b0]) + cmplx.Abs(fftBuf[b1])) * 0.5
		}
		used++
	}
	if used == 0 {
		used = 1
	}

	invN := 1.0 / float64(FFTSize)
	for i := 0; i < half; i++ {
		mag := powBuf[i] / float64(used) * invN
		if mag < 1e-12 {
			mag = 1e-12
		}
		dbOut[i] = float32(20 * math.Log10(mag))
	}
}

// fftInPlace performs an in-place radix-2 Cooley-Tukey FFT.
func fftInPlace(x []complex128) {
	n := len(x)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation.
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}

	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		wLen := cmplx.Rect(1, angle)
		half := length / 2
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for k := 0; k < half; k++ {
				u := x[i+k]
				v := x[i+k+half] * w
				x[i+k] = u + v
				x[i+k+half] = u - v
				w *= wLen
			}
		}
	}
}
