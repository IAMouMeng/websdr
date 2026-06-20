package dsp

import "math"

// SpectralNR is a streaming spectral noise-reduction processor for voice. It
// runs a short-time Fourier transform over the audio, estimates a per-bin
// noise floor, and attenuates each bin with a decision-directed (Ephraim-Malah)
// Wiener gain. The gain is driven by a heavily smoothed *a priori* SNR rather
// than the noisy instantaneous power, which keeps speech intact while avoiding
// the random "musical noise" burbling that plain spectral subtraction leaves in
// pauses. It runs at a fixed audio rate (48 kHz) so its frame timing and
// frequency resolution are stable.
//
// Process works fully in place and returns the same number of samples it was
// given, carrying its overlap-add state across calls. There is a fixed startup
// latency of one frame (a brief mute) the first time audio flows.
type SpectralNR struct {
	inBuf    []float32 // samples not yet framed
	outBuf   []float32 // finished samples waiting to be drained
	ola      []float32 // overlap-add accumulator, length frameSize
	win      []float64 // sqrt-Hann analysis/synthesis window
	colaNorm float64   // overlap-add normalization for the chosen window/hop
	fft      []complex128
	noise    []float64 // per-bin noise power estimate
	smooth   []float64 // per-bin smoothed power (for noise tracking)
	prevS    []float64 // per-bin previous enhanced power (decision-directed state)
	primed   bool
}

const (
	nrFrame = 1024        // STFT frame size at 48 kHz (~21 ms, ~47 Hz/bin)
	nrHop   = nrFrame / 4 // 75% overlap → smoother, lower-artifact reconstruction
)

func (n *SpectralNR) init() {
	if n.win != nil {
		return
	}
	n.win = make([]float64, nrFrame)
	for i := range n.win {
		// sqrt-Hann: applied on both analysis and synthesis, the product is a
		// Hann window, whose 50%-overlap sum is constant → unity reconstruction.
		h := 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(nrFrame-1)))
		n.win[i] = math.Sqrt(h)
	}
	// COLA normalization: with sqrt-Hann on both analysis and synthesis, each
	// output sample is scaled by the sum of the overlapping windows squared.
	// Dividing by that constant gives unity-gain reconstruction for any hop.
	n.colaNorm = 0
	for j := (nrFrame / 2) % nrHop; j < nrFrame; j += nrHop {
		n.colaNorm += n.win[j] * n.win[j]
	}

	n.ola = make([]float32, nrFrame)
	n.fft = make([]complex128, nrFrame)
	n.noise = make([]float64, nrFrame)
	n.smooth = make([]float64, nrFrame)
	n.prevS = make([]float64, nrFrame)
}

// Reset clears all carried state so the next block starts fresh.
func (n *SpectralNR) Reset() {
	n.inBuf = n.inBuf[:0]
	n.outBuf = n.outBuf[:0]
	n.primed = false
	for i := range n.ola {
		n.ola[i] = 0
	}
	for i := range n.noise {
		n.noise[i] = 0
		n.smooth[i] = 0
		n.prevS[i] = 0
	}
}

// Process attenuates noise in buf in place. level in [0,1] sets aggressiveness:
// 0 is gentle, 1 subtracts hard with a low gain floor.
func (n *SpectralNR) Process(buf []float32, level float64) {
	if len(buf) == 0 {
		return
	}
	n.init()
	if level < 0 {
		level = 0
	} else if level > 1 {
		level = 1
	}
	// noiseBias mildly over-estimates the noise floor (more bias → quieter
	// gaps but also dimmer voice); gmin is the residual gain floor. logMMSE
	// keeps musical noise low on its own, so suppression can go fairly deep
	// without gutting speech.
	noiseBias := 1.0 + 3.0*level // 1.0 .. 4.0
	gmin := 0.10 - 0.09*level    // ~ -20 dB .. -40 dB

	n.inBuf = append(n.inBuf, buf...)
	for len(n.inBuf) >= nrFrame {
		n.processFrame(noiseBias, gmin)
		n.inBuf = n.inBuf[nrHop:]
	}
	n.drain(buf)
}

// processFrame applies the log-spectral-amplitude MMSE estimator (Ephraim &
// Malah 1985) with a decision-directed a priori SNR. logMMSE minimizes the
// error of the log magnitude, which matches perceived loudness — it gives
// markedly clearer speech and lower musical noise than spectral subtraction or
// plain Wiener filtering.
func (n *SpectralNR) processFrame(noiseBias, gmin float64) {
	const (
		powSmooth = 0.8    // power EMA for noise tracking (steadier estimate)
		noiseRise = 1.0008 // noise floor very slow upward tracking per hop, so
		//                    continuous speech can't pull the floor into itself
		ddAlpha = 0.98 // decision-directed a priori SNR smoothing
	)
	for i := 0; i < nrFrame; i++ {
		n.fft[i] = complex(float64(n.inBuf[i])*n.win[i], 0)
	}
	fftInPlace(n.fft)
	for k := 0; k < nrFrame; k++ {
		re, im := real(n.fft[k]), imag(n.fft[k])
		p := re*re + im*im
		n.smooth[k] = powSmooth*n.smooth[k] + (1-powSmooth)*p
		// Track the noise floor: follow the smoothed power down instantly,
		// let it rise only slowly so brief speech doesn't raise the floor.
		if n.noise[k] == 0 || n.smooth[k] < n.noise[k] {
			n.noise[k] = n.smooth[k]
		} else {
			n.noise[k] *= noiseRise
		}
		nz := n.noise[k]*noiseBias + 1e-12

		gamma := p / nz // a posteriori SNR
		post := gamma - 1
		if post < 0 {
			post = 0
		}
		// Decision-directed a priori SNR: blend the previous enhanced power
		// with the current instantaneous estimate (α weighted toward history).
		xi := ddAlpha*(n.prevS[k]/nz) + (1-ddAlpha)*post
		if xi < 1e-6 {
			xi = 1e-6
		}
		// logMMSE gain: G = (ξ/(1+ξ))·exp(½·E1(v)), v = (ξ/(1+ξ))·γ.
		ratio := xi / (1 + xi)
		v := ratio * gamma
		g := ratio * math.Exp(0.5*expInt(v))
		if g < gmin {
			g = gmin
		} else if g > 1 {
			g = 1
		}
		n.prevS[k] = g * g * p // enhanced power, state for the next frame
		n.fft[k] = complex(re*g, im*g)
	}
	ifftInPlace(n.fft)
	// Synthesis window + overlap-add (COLA-normalized), then emit the first hop.
	inv := 1.0 / n.colaNorm
	for i := 0; i < nrFrame; i++ {
		n.ola[i] += float32(real(n.fft[i]) * n.win[i] * inv)
	}
	n.outBuf = append(n.outBuf, n.ola[:nrHop]...)
	copy(n.ola, n.ola[nrHop:])
	for i := nrFrame - nrHop; i < nrFrame; i++ {
		n.ola[i] = 0
	}
}

// drain copies finished samples into buf. Until one frame has been buffered
// (startup priming) it outputs silence without consuming, giving a fixed
// one-frame latency and a cushion that prevents steady-state underruns.
func (n *SpectralNR) drain(buf []float32) {
	if !n.primed {
		if len(n.outBuf) >= nrFrame {
			n.primed = true
		} else {
			for i := range buf {
				buf[i] = 0
			}
			return
		}
	}
	if len(n.outBuf) >= len(buf) {
		copy(buf, n.outBuf[:len(buf)])
		n.outBuf = n.outBuf[len(buf):]
		return
	}
	// Rare underrun: emit what we have, pad the rest with silence.
	k := copy(buf, n.outBuf)
	for i := k; i < len(buf); i++ {
		buf[i] = 0
	}
	n.outBuf = n.outBuf[:0]
}

// expInt computes the exponential integral E1(x) = ∫_x^∞ e^-t/t dt for x > 0,
// using the standard Abramowitz & Stegun rational approximations (7.1.79 /
// 5.1.56). Used by the logMMSE gain.
func expInt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x < 1 {
		return -math.Log(x) - 0.57721566 + x*(0.99999193+x*(-0.24991055+
			x*(0.05519968+x*(-0.00976004+x*0.00107857))))
	}
	num := x*x + 2.334733*x + 0.250621
	den := x*x + 3.330657*x + 1.681534
	return (math.Exp(-x) / x) * (num / den)
}

// ifftInPlace inverts fftInPlace using the conjugation identity:
// ifft(x) = conj(fft(conj(x))) / N.
func ifftInPlace(x []complex128) {
	for i := range x {
		x[i] = complex(real(x[i]), -imag(x[i]))
	}
	fftInPlace(x)
	invN := 1.0 / float64(len(x))
	for i := range x {
		x[i] = complex(real(x[i])*invN, -imag(x[i])*invN)
	}
}
