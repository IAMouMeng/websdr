package dsp

import "math"

// FIR is a windowed-sinc low-pass filter. It carries its delay line across
// calls so consecutive blocks stay continuous (no per-block transient), and
// it redesigns its kernel automatically when the cutoff or sample rate
// changes. The same instance filters either a complex (channel selectivity)
// or a real (audio) stream; pick the matching Process method and keep using
// it for that stream.
//
// Unlike a moving-average ("boxcar") filter, the windowed-sinc kernel has a
// flat passband out to the cutoff and real stopband rejection, so a requested
// bandwidth is actually delivered instead of being rolled off early.
type FIR struct {
	taps   []float64
	histC  []complex128 // delay line for complex streams, len = len(taps)-1
	histR  []float32    // delay line for real streams
	outC   []complex128 // scratch output (filtering is not in place)
	outR   []float32
	cutoff float64
	rate   float64
}

// Reset clears the delay line so the next block starts from silence.
func (f *FIR) Reset() {
	for i := range f.histC {
		f.histC[i] = 0
	}
	for i := range f.histR {
		f.histR[i] = 0
	}
}

// ensure (re)designs the kernel if the cutoff or rate changed since last call.
func (f *FIR) ensure(cutoffHz, rate float64) {
	if f.taps != nil && cutoffHz == f.cutoff && rate == f.rate {
		return
	}
	f.cutoff = cutoffHz
	f.rate = rate
	f.taps = designLowPass(cutoffHz, rate)
	f.histC = make([]complex128, len(f.taps)-1)
	f.histR = make([]float32, len(f.taps)-1)
}

// designLowPass builds a Hann-windowed sinc low-pass kernel with unity DC
// gain. The tap count scales with rate/cutoff to keep the transition band a
// roughly fixed fraction of the cutoff, bounded so cost stays reasonable.
func designLowPass(cutoffHz, rate float64) []float64 {
	fc := cutoffHz / rate // normalized passband edge, cycles/sample
	if fc <= 0 {
		fc = 1e-4
	}
	if fc > 0.49 {
		fc = 0.49
	}
	n := int(4 / fc) // ~4/fc taps → transition ≈ 0.78·cutoff
	if n < 15 {
		n = 15
	}
	if n > 255 {
		n = 255
	}
	if n%2 == 0 {
		n++
	}
	taps := make([]float64, n)
	mid := (n - 1) / 2
	var sum float64
	for i := 0; i < n; i++ {
		k := float64(i - mid)
		var h float64
		if k == 0 {
			h = 2 * fc
		} else {
			h = math.Sin(2*math.Pi*fc*k) / (math.Pi * k)
		}
		// Hann window
		h *= 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1))
		taps[i] = h
		sum += h
	}
	for i := range taps {
		taps[i] /= sum
	}
	return taps
}

// ProcessComplex low-passes a complex baseband stream in place, using cutoffHz
// as the one-sided cutoff (so the total passband width is 2·cutoffHz).
func (f *FIR) ProcessComplex(buf []complex128, cutoffHz, rate float64) {
	f.ensure(cutoffHz, rate)
	taps, m, hist := f.taps, len(f.taps), f.histC
	h := m - 1
	n := len(buf)
	if cap(f.outC) < n {
		f.outC = make([]complex128, n)
	}
	out := f.outC[:n]
	for i := 0; i < n; i++ {
		var re, im float64
		for j := 0; j < m; j++ {
			idx := i - h + j
			var s complex128
			if idx < 0 {
				s = hist[h+idx]
			} else {
				s = buf[idx]
			}
			t := taps[j]
			re += t * real(s)
			im += t * imag(s)
		}
		out[i] = complex(re, im)
	}
	f.saveHistC(buf)
	copy(buf, out)
}

// DecimateComplex low-passes and downsamples a complex stream in a single
// pass: it writes every factor-th filtered sample to dst and returns the count
// written. cutoffHz is the one-sided passband edge at the INPUT rate; pick it
// below outputRate/2 so out-of-band signals are attenuated before they fold
// (alias) onto baseband. Only the kept output samples are convolved, so cost
// is 1/factor of filtering at the full rate. The delay line carries across
// calls exactly like ProcessComplex, so consecutive blocks stay continuous.
func (f *FIR) DecimateComplex(buf, dst []complex128, factor int, cutoffHz, rate float64) int {
	if factor <= 1 {
		f.ProcessComplex(buf, cutoffHz, rate)
		n := len(buf)
		if n > len(dst) {
			n = len(dst)
		}
		copy(dst[:n], buf[:n])
		return n
	}
	f.ensure(cutoffHz, rate)
	taps, m, hist := f.taps, len(f.taps), f.histC
	h := m - 1
	n := len(buf)
	out := 0
	for i := 0; i < n && out < len(dst); i += factor {
		var re, im float64
		for j := 0; j < m; j++ {
			idx := i - h + j
			var s complex128
			if idx < 0 {
				s = hist[h+idx]
			} else {
				s = buf[idx]
			}
			t := taps[j]
			re += t * real(s)
			im += t * imag(s)
		}
		dst[out] = complex(re, im)
		out++
	}
	f.saveHistC(buf)
	return out
}

// ProcessReal low-passes a real audio stream in place at the given cutoff.
func (f *FIR) ProcessReal(buf []float32, cutoffHz, rate float64) {
	if cutoffHz <= 0 {
		return
	}
	f.ensure(cutoffHz, rate)
	taps, m, hist := f.taps, len(f.taps), f.histR
	h := m - 1
	n := len(buf)
	if cap(f.outR) < n {
		f.outR = make([]float32, n)
	}
	out := f.outR[:n]
	for i := 0; i < n; i++ {
		var acc float64
		for j := 0; j < m; j++ {
			idx := i - h + j
			var s float32
			if idx < 0 {
				s = hist[h+idx]
			} else {
				s = buf[idx]
			}
			acc += taps[j] * float64(s)
		}
		out[i] = float32(acc)
	}
	f.saveHistR(buf)
	copy(buf, out)
}

// saveHistC stores the last len(histC) input samples for the next block.
func (f *FIR) saveHistC(buf []complex128) {
	h := len(f.histC)
	if len(buf) >= h {
		copy(f.histC, buf[len(buf)-h:])
		return
	}
	copy(f.histC, f.histC[len(buf):])
	copy(f.histC[h-len(buf):], buf)
}

// saveHistR stores the last len(histR) input samples for the next block.
func (f *FIR) saveHistR(buf []float32) {
	h := len(f.histR)
	if len(buf) >= h {
		copy(f.histR, buf[len(buf)-h:])
		return
	}
	copy(f.histR, f.histR[len(buf):])
	copy(f.histR[h-len(buf):], buf)
}
