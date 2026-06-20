package dsp

import (
	"math"
	"math/cmplx"
)

// FMDemod performs FM demodulation via complex-conjugate product (quadrature
// discriminator), which is lower-noise than per-sample atan2. The previous
// sample is carried across calls in prev.
func FMDemod(samples []complex128, deviation, sampleRate float64, prev *complex128, audio []float32) int {
	n := len(samples)
	scale := float32(sampleRate / (2 * math.Pi * deviation))
	last := *prev
	for i := 0; i < n; i++ {
		s := samples[i]
		if i == 0 && cmplx.Abs(last) < 1e-12 {
			last = s
		}
		prod := cmplx.Conj(last) * s
		last = s
		audio[i] = float32(imag(prod)) * scale
	}
	*prev = last
	return n
}

// DeemphasisFM applies 75µs broadcast FM de-emphasis (single-pole low-pass).
func DeemphasisFM(audio []float32, sampleRate float64, state *float32) {
	tau := 75e-6
	alpha := float32(math.Exp(-1.0 / (sampleRate * tau)))
	for i := range audio {
		*state = alpha*(*state) + (1-alpha)*audio[i]
		audio[i] = *state
	}
}

// AMDemod performs AM demodulation by envelope detection.
func AMDemod(samples []complex128, audio []float32) int {
	n := len(samples)
	if n > len(audio) {
		n = len(audio)
	}
	for i := 0; i < n; i++ {
		audio[i] = float32(cmplx.Abs(samples[i]))
	}
	return n
}

// USBDemod demodulates upper sideband (real part at zero IF).
func USBDemod(samples []complex128, audio []float32) int {
	n := len(samples)
	if n > len(audio) {
		n = len(audio)
	}
	for i := 0; i < n; i++ {
		audio[i] = float32(real(samples[i]))
	}
	return n
}

// DSBDemod demodulates double-sideband (product detector real output). It is
// also used for the RAW passthrough mode, which simply skips the channel
// filter so the full decimated bandwidth is heard.
func DSBDemod(samples []complex128, audio []float32) int {
	n := len(samples)
	if n > len(audio) {
		n = len(audio)
	}
	for i := 0; i < n; i++ {
		audio[i] = float32(real(samples[i]))
	}
	return n
}

// LSBDemod demodulates lower sideband.
func LSBDemod(samples []complex128, audio []float32) int {
	n := len(samples)
	if n > len(audio) {
		n = len(audio)
	}
	for i := 0; i < n; i++ {
		audio[i] = float32(-imag(samples[i]))
	}
	return n
}
