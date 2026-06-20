package receiver

import (
	"fmt"
	"time"

	"hz.tools/rf"
	"hz.tools/sdr"

	"github.com/iamoumeng/websdr/internal/dsp"
	"github.com/iamoumeng/websdr/internal/protocol"
)

const (
	lrptCenterFreq     = 137_900_000
	lrptSampleRate     = 2_048_000
	lrptDefaultFilter  = 150_000
	lrptEmitInterval   = 500 * time.Millisecond
)

// LRPTPayload is the LRPT listen page snapshot.
type LRPTPayload struct {
	FreqHz     uint64  `json:"freqHz"`
	Freq       string  `json:"freq"`
	Strength   int     `json:"strength"`
	Metric     string  `json:"metric,omitempty"`
	Locked     bool    `json:"locked"`
	Listening  bool    `json:"listening"`
	ElapsedSec float64 `json:"elapsedSec"`
}

type lrptListenState struct {
	active    bool
	freqHz    uint64
	offsetHz  float64
	strength  int
	metric    string
	locked    bool
	startedAt time.Time
	lastEmit  time.Time
}

func (r *Receiver) initLRPTListen() {
	r.lrptListen = lrptListenState{}
}

func (r *Receiver) SetLRPTListen(on bool, freqHz uint64) {
	r.enqueue(func() {
		if on {
			if freqHz < 136_000_000 || freqHz > 138_500_000 {
				freqHz = lrptCenterFreq
			}
			freqHz = protocol.SnapLRPTDownlink(freqHz)
			r.lrptListen.active = true
			r.lrptListen.freqHz = freqHz
			r.lrptListen.offsetHz = float64(freqHz) - float64(lrptCenterFreq)
			r.lrptListen.metric = ""
			r.lrptListen.locked = false
			r.lrptListen.startedAt = time.Now()
			r.lrptListen.lastEmit = time.Time{}
			if r.sdr != nil {
				r.forceNormalTunerModeIfNeeded()
				_ = r.sdr.SetSampleRate(lrptSampleRate)
				_ = r.sdr.SetCenterFrequency(rf.Hz(lrptCenterFreq))
			}
		} else {
			r.lrptListen.active = false
		}
	})
}

func (r *Receiver) processLRPT(samples sdr.SamplesU8) {
	if !r.lrptListen.active {
		return
	}
	n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
	if n < dsp.FFTSize {
		r.maybeEmitLRPT()
		return
	}
	iq := r.iqBuf[:n]
	cfg := r.lrptDisplayConfig()
	r.emitSpectrum(iq, cfg)
	r.processAudio(iq, cfg, float64(lrptSampleRate))

	dsp.PowerSpectrum(iq, spectrumSegs, r.fftBuf, r.window, r.powBuf, r.specOut)
	peak := protocol.DetectPeaks(r.specOut, lrptCenterFreq, lrptSampleRate)
	if len(peak) > 0 {
		p := peak[0]
		r.lrptListen.strength = int(p.PowerDB)
	}
	symRate := protocol.EstimateSymbolRatePublic(iq, float64(lrptSampleRate), r.lrptListen.offsetHz)
	if symRate >= 50_000 && symRate <= 120_000 {
		r.lrptListen.locked = true
		r.lrptListen.metric = fmt.Sprintf("%.0f sym/s", symRate)
	} else {
		r.lrptListen.locked = false
		if symRate > 0 {
			r.lrptListen.metric = fmt.Sprintf("%.0f sym/s?", symRate)
		}
	}
	r.maybeEmitLRPT()
}

func (r *Receiver) maybeEmitLRPT() {
	if !r.lrptListen.active {
		return
	}
	now := time.Now()
	if !r.lrptListen.lastEmit.IsZero() && now.Sub(r.lrptListen.lastEmit) < lrptEmitInterval {
		return
	}
	r.lrptListen.lastEmit = now
	freqHz := r.lrptListen.freqHz
	if freqHz == 0 {
		freqHz = lrptCenterFreq
	}
	r.pushDecode(DecodeFrame{
		Service: ServiceLRPT,
		LRPT: &LRPTPayload{
			FreqHz:     freqHz,
			Freq:       fmt.Sprintf("%.3f MHz", float64(freqHz)/1e6),
			Strength:   r.lrptListen.strength,
			Metric:     r.lrptListen.metric,
			Locked:     r.lrptListen.locked,
			Listening:  true,
			ElapsedSec: now.Sub(r.lrptListen.startedAt).Seconds(),
		},
	})
}

func (r *Receiver) lrptDisplayConfig() Config {
	freqHz := r.lrptListen.freqHz
	if freqHz == 0 {
		freqHz = lrptCenterFreq
	}
	bw := r.config.FilterBW
	if bw < 80_000 || bw > 250_000 {
		bw = lrptDefaultFilter
	}
	return Config{
		Service:    ServiceLRPT,
		CenterFreq: lrptCenterFreq,
		TuneFreq:   freqHz,
		SampleRate: lrptSampleRate,
		FilterBW:   bw,
		Mode:       ModeUSB,
	}
}

func (r *Receiver) lrptTuning() (center uint64, rate uint) {
	return lrptCenterFreq, lrptSampleRate
}
