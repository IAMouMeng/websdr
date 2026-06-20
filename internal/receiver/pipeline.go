package receiver

import (
	"hz.tools/sdr"

	"github.com/iamoumeng/websdr/internal/dsp"
)

const (
	maxBlockSamples = 131072
	minBlockSamples = 8192
	spectrumSegs    = 4 // FFT segments averaged per spectrum frame
)

// blockSamples sizes the IQ read block so that, at the given sample rate,
// blocks arrive at roughly spectrumFPS frames per second.
func blockSamples(sampleRate uint) int {
	n := int(sampleRate) / spectrumFPS
	if n < minBlockSamples {
		n = minBlockSamples
	}
	if n > maxBlockSamples {
		n = maxBlockSamples
	}
	return n
}

func (r *Receiver) processBlock(samples sdr.SamplesU8) {
	cfg := r.Config()
	switch cfg.Service {
	case ServiceADSB:
		r.processADSB(samples)
	case ServiceAIS:
		n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
		if n == 0 {
			return
		}
		r.processAIS(r.iqBuf[:n], float64(aisSampleRate))
	case ServiceProtocol:
		r.processProtocol(samples)
	case ServiceAPT:
		r.processAPT(samples)
	case ServiceLRPT:
		r.processLRPT(samples)
	case ServiceMeteor:
		r.processMeteor(samples)
	default:
		n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
		if n == 0 {
			return
		}
		r.emitSpectrum(r.iqBuf[:n], cfg)
		r.processAudio(r.iqBuf[:n], cfg, float64(cfg.SampleRate))
	}
}

// emitSpectrum computes a smoothed power spectrum and pushes a frame. The
// per-bin exponential average keeps the waterfall from flickering.
func (r *Receiver) emitSpectrum(iq []complex128, cfg Config) {
	if len(iq) < dsp.FFTSize {
		return
	}
	dsp.PowerSpectrum(iq, spectrumSegs, r.fftBuf, r.window, r.powBuf, r.specOut)

	const smooth = 0.5
	const scale = 255.0 / (specDBMax - specDBMin)
	data := make([]byte, len(r.specOut))
	for i, v := range r.specOut {
		r.specSmooth[i] = r.specSmooth[i]*(1-smooth) + v*smooth
		// Quantize real dB onto a byte over [specDBMin, specDBMax].
		q := (float64(r.specSmooth[i]) - specDBMin) * scale
		if q < 0 {
			q = 0
		} else if q > 255 {
			q = 255
		}
		data[i] = byte(q)
	}

	frame := SpectrumFrame{
		Data:       data,
		CenterFreq: cfg.CenterFreq,
		TuneFreq:   cfg.TuneFreq,
		SampleRate: cfg.SampleRate,
		FilterBW:   cfg.FilterBW,
	}
	select {
	case r.spectrumCh <- frame:
	default:
	}
}

func (r *Receiver) processAudio(iq []complex128, cfg Config, sr float64) {
	// Shift the signal sitting at +offset (tune above center) down to
	// baseband: multiply by exp(-j·2π·offset·n/sr), hence the negated offset.
	offset := float64(cfg.TuneFreq) - float64(cfg.CenterFreq)
	work := r.workBuf[:len(iq)]
	copy(work, iq)
	dsp.MixDown(work, -offset, sr, &r.mixPhase)

	factor := dsp.DecimFactorForRate(cfg.SampleRate)
	decimSR := sr / float64(factor)
	// Anti-alias before downsampling: without a low-pass limiting the signal to
	// the post-decimation Nyquist band (decimSR/2), out-of-band stations fold
	// onto the tuned channel and play on top of it. The 0.45 factor leaves a
	// guard band below Nyquist for the filter's transition region. Done in one
	// pass with the decimation so only the kept samples are convolved.
	var decimN int
	if factor > 1 {
		decimN = r.aaFilt.DecimateComplex(work, r.decimBuf, factor, decimSR*0.45, sr)
	} else {
		decimN = decimateInPlace(work, r.decimBuf, factor)
	}
	if decimN == 0 {
		return
	}
	if decimN > len(r.audioF) {
		decimN = len(r.audioF)
	}
	ch := r.decimBuf[:decimN]

	// Channel (selectivity) filter sized from the requested bandwidth: the
	// complex baseband channel spans ±FilterBW/2 around the tuned frequency,
	// so the one-sided cutoff is half the bandwidth. RAW is a passthrough
	// mode and deliberately skips it.
	if cfg.Mode != ModeRAW {
		r.chFilt.ProcessComplex(ch, cfg.FilterBW/2, decimSR)
	}

	af := r.audioF[:decimN]
	if cfg.Service == ServiceAPT {
		dsp.FMDemod(ch, 17000, decimSR, &r.fmPrev, af)
		r.audFilt.ProcessReal(af, 4000, decimSR)
	} else if cfg.Service == ServiceLRPT || cfg.Service == ServiceMeteor {
		dsp.USBDemod(ch, af)
		r.audFilt.ProcessReal(af, cfg.FilterBW, decimSR)
	} else {
		switch cfg.Mode {
		case ModeAM:
			dsp.AMDemod(ch, af)
			r.audFilt.ProcessReal(af, 5000, decimSR)
		case ModeUSB:
			dsp.USBDemod(ch, af)
			r.audFilt.ProcessReal(af, cfg.FilterBW, decimSR)
		case ModeLSB:
			dsp.LSBDemod(ch, af)
			r.audFilt.ProcessReal(af, cfg.FilterBW, decimSR)
		case ModeDSB:
			dsp.DSBDemod(ch, af)
			r.audFilt.ProcessReal(af, cfg.FilterBW, decimSR)
		case ModeRAW:
			dsp.DSBDemod(ch, af)
			r.audFilt.ProcessReal(af, 15000, decimSR)
		case ModeCW:
			cwIQ := r.cwBuf[:decimN]
			copy(cwIQ, ch)
			dsp.MixDown(cwIQ, cfg.CWPitch, decimSR, &r.bfoPhase)
			dsp.AMDemod(cwIQ, af)
			r.audFilt.ProcessReal(af, cfg.FilterBW, decimSR)
		case ModeFM:
			dsp.FMDemod(ch, 5000, decimSR, &r.fmPrev, af)
			r.audFilt.ProcessReal(af, 4000, decimSR)
		default: // WFM
			dsp.FMDemod(ch, 75000, decimSR, &r.fmPrev, af)
			dsp.DeemphasisFM(af, decimSR, &r.deAcc)
			r.audFilt.ProcessReal(af, 15000, decimSR)
		}
	}

	dsp.DCBlock(af, &r.dcAcc)

	// Fractional resample to exactly AudioRate, then level and pack to PCM.
	pcmN := r.resamp.Resample(af, decimSR, float64(dsp.AudioRate), r.pcmF)
	if pcmN == 0 {
		return
	}
	pcm := r.pcmF[:pcmN]
	// Spectral noise reduction runs at the fixed 48 kHz audio rate so its frame
	// timing and frequency resolution stay constant, and before AGC so the gain
	// tracks the cleaned signal.
	if cfg.NR {
		r.nr.Process(pcm, cfg.NRLevel)
	}
	r.agc.Process(pcm)
	ic := dsp.Float32ToInt16(pcm, r.audioI)

	r.audioAccum = append(r.audioAccum, r.audioI[:ic]...)
	r.flushAudio()
}

func (r *Receiver) flushAudio() {
	for len(r.audioAccum) >= audioChunkSamples {
		chunk := make([]int16, audioChunkSamples)
		copy(chunk, r.audioAccum[:audioChunkSamples])
		r.audioAccum = r.audioAccum[audioChunkSamples:]

		frame := AudioFrame{PCM: chunk, Rate: dsp.AudioRate}
		select {
		case r.audioCh <- frame:
		default:
			// Channel full: drop the oldest frame and retry so the
			// accumulator can't grow without bound.
			select {
			case <-r.audioCh:
			default:
			}
			select {
			case r.audioCh <- frame:
			default:
			}
		}
	}
	// Keep capacity bounded after repeated reslicing.
	if cap(r.audioAccum) > audioChunkSamples*8 && len(r.audioAccum) < audioChunkSamples {
		r.audioAccum = append(make([]int16, 0, audioChunkSamples*4), r.audioAccum...)
	}
}

// decimateInPlace keeps every factor-th sample of src into dst.
func decimateInPlace(src, dst []complex128, factor int) int {
	if factor <= 1 {
		n := len(src)
		if n > len(dst) {
			n = len(dst)
		}
		copy(dst[:n], src[:n])
		return n
	}
	n := len(src) / factor
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		dst[i] = src[i*factor]
	}
	return n
}
