// Package dsp implements the signal-processing primitives used by the WebSDR
// receiver: IQ conversion, frequency mixing, FFT spectrum, demodulators,
// filters and resampling.
package dsp

import (
	"math"
	"math/cmplx"
)

const (
	// FFTSize is the number of bins used for each spectrum FFT.
	FFTSize = 2048
	// AudioRate is the PCM sample rate delivered to the browser.
	AudioRate = 48000
)

// DecimFactorForRate picks an integer decimation factor that keeps the
// per-channel IQ processing rate in a comfortable range (~250-400 kHz).
func DecimFactorForRate(sampleRate uint) int {
	switch {
	case sampleRate <= 300000:
		return 1
	case sampleRate <= 1200000:
		return 4
	default:
		return 8
	}
}

// SamplesU8ToComplex converts RTL-SDR unsigned-8-bit IQ samples to normalized
// complex128 in dst, returning the number of samples written.
func SamplesU8ToComplex(samples [][2]uint8, dst []complex128) int {
	n := len(samples)
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		re := (float64(samples[i][0]) - 127.5) / 127.5
		im := (float64(samples[i][1]) - 127.5) / 127.5
		dst[i] = complex(re, im)
	}
	return n
}

// MixDown shifts IQ samples by freqOffset Hz, carrying the oscillator phase
// across calls. It is a no-op for a zero offset.
func MixDown(samples []complex128, freqOffset, sampleRate float64, phase *float64) {
	if freqOffset == 0 {
		return
	}
	phaseInc := 2 * math.Pi * freqOffset / sampleRate
	p := *phase
	for i, s := range samples {
		samples[i] = s * cmplx.Rect(1, p)
		p += phaseInc
		if p > math.Pi {
			p -= 2 * math.Pi
		} else if p < -math.Pi {
			p += 2 * math.Pi
		}
	}
	*phase = p
}

// DCBlock removes a slowly-varying DC offset (single-pole high-pass).
func DCBlock(src []float32, acc *float32) {
	const alpha float32 = 0.999
	for i := range src {
		*acc = alpha*(*acc) + (1-alpha)*src[i]
		src[i] -= *acc
	}
}

// Float32ToInt16 converts clamped float audio [-1,1] to int16 PCM.
func Float32ToInt16(src []float32, dst []int16) int {
	n := len(src)
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		v := src[i]
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		dst[i] = int16(v * 32767)
	}
	return n
}

// AGC applies smooth automatic gain control. The peak estimate decays slowly
// so the gain stays steady between transients instead of pumping per block.
type AGC struct {
	peak float32
}

// Process normalizes buf in place toward a fixed reference level.
func (a *AGC) Process(buf []float32) {
	const ref float32 = 0.5
	const decay float32 = 0.99994 // ~0.35s time constant at 48 kHz
	const maxGain float32 = 40
	const peakFloor float32 = 1e-6
	for i, v := range buf {
		av := v
		if av < 0 {
			av = -av
		}
		if av > a.peak {
			a.peak = av
		} else {
			a.peak *= decay
		}
		peak := a.peak
		if peak < peakFloor {
			peak = peakFloor
		}
		g := ref / peak
		if g > maxGain {
			g = maxGain
		}
		out := v * g
		if out > 1 {
			out = 1
		} else if out < -1 {
			out = -1
		}
		buf[i] = out
	}
}

// Resampler performs fractional linear resampling, carrying its phase and the
// trailing sample across calls so block boundaries stay continuous. This is
// what lets an arbitrary channel rate land on exactly AudioRate.
type Resampler struct {
	pos  float64 // fractional read position relative to the current block
	prev float32 // last sample of the previous block (virtual index -1)
	have bool
}

// Reset clears the resampler state.
func (rs *Resampler) Reset() { rs.have = false; rs.pos = 0; rs.prev = 0 }

// Resample reads in (sampled at inRate) and writes interpolated output at
// outRate into dst, returning the number of output samples produced.
func (rs *Resampler) Resample(in []float32, inRate, outRate float64, dst []float32) int {
	if len(in) == 0 || inRate <= 0 || outRate <= 0 {
		return 0
	}
	if !rs.have {
		rs.prev = in[0]
		rs.pos = 0
		rs.have = true
	}
	step := inRate / outRate
	pos := rs.pos
	o := 0
	for o < len(dst) {
		base := int(math.Floor(pos))
		if base+1 >= len(in) {
			break
		}
		frac := float32(pos - float64(base))
		var a float32
		if base < 0 {
			a = rs.prev
		} else {
			a = in[base]
		}
		b := in[base+1]
		dst[o] = a*(1-frac) + b*frac
		o++
		pos += step
	}
	rs.prev = in[len(in)-1]
	rs.pos = pos - float64(len(in)) // re-base onto the next block's origin
	return o
}
