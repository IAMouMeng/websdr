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
	aptCenterFreq      = 137_100_000
	aptSampleRate      = 1_024_000
	aptDefaultFilterBW = 40_000
	aptEmitInterval    = 500 * time.Millisecond
	aptImageInterval   = 2 * time.Second
	maxAPTListenAudio  = int(120 * protocol.APTAudioRate) // up to ~2 min
)

// APTPayload is the dedicated APT listen page snapshot.
type APTPayload struct {
	FreqHz     uint64  `json:"freqHz"`
	Freq       string  `json:"freq"`
	Strength   int     `json:"strength"`
	Lines      int     `json:"lines"`
	Image      string  `json:"image,omitempty"`
	Decoded    bool    `json:"decoded"`
	Metric     string  `json:"metric,omitempty"`
	Listening  bool    `json:"listening"`
	ElapsedSec float64 `json:"elapsedSec"`
}

type aptListenState struct {
	active      bool
	freqHz      uint64
	offsetHz    float64
	audio       []float32
	strength    int
	decoded     bool
	metric      string
	image       string
	lines       int
	startedAt   time.Time
	lastEmit    time.Time
	lastImageAt time.Time
}

func (r *Receiver) initAPTListen() {
	r.aptListen = aptListenState{
		audio: make([]float32, 0, maxAPTListenAudio/8),
	}
}

// SetAPTListen starts or stops dedicated APT decoding on 137 MHz.
func (r *Receiver) SetAPTListen(on bool, freqHz uint64) {
	r.enqueue(func() {
		if on {
			if freqHz < 136_000_000 || freqHz > 138_000_000 {
				freqHz = aptCenterFreq
			}
			freqHz = protocol.SnapAPTDownlink(freqHz)
			r.aptListen.active = true
			r.aptListen.freqHz = freqHz
			r.aptListen.offsetHz = float64(freqHz) - float64(aptCenterFreq)
			r.aptListen.audio = r.aptListen.audio[:0]
			r.aptListen.image = ""
			r.aptListen.lines = 0
			r.aptListen.decoded = false
			r.aptListen.metric = ""
			r.aptListen.startedAt = time.Now()
			r.aptListen.lastEmit = time.Time{}
			r.aptListen.lastImageAt = time.Time{}
			if r.sdr != nil {
				r.forceNormalTunerModeIfNeeded()
				_ = r.sdr.SetSampleRate(aptSampleRate)
				_ = r.sdr.SetCenterFrequency(rf.Hz(aptCenterFreq))
			}
		} else {
			r.aptListen.active = false
		}
	})
}

func (r *Receiver) processAPT(samples sdr.SamplesU8) {
	if !r.aptListen.active {
		return
	}
	n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
	if n < dsp.FFTSize {
		r.maybeEmitAPT()
		return
	}
	iq := r.iqBuf[:n]
	cfg := r.aptDisplayConfig()
	r.emitSpectrum(iq, cfg)
	r.processAudio(iq, cfg, float64(aptSampleRate))

	chunk := protocol.FMAudioFromIQ(iq, aptSampleRate, r.aptListen.offsetHz)
	r.aptListen.audio = protocol.AppendFMAudio(r.aptListen.audio, chunk, maxAPTListenAudio)

	peak := protocol.DetectPeaks(r.specOut, aptCenterFreq, aptSampleRate)
	if len(peak) > 0 {
		p := peak[0]
		r.aptListen.strength = int(p.PowerDB)
	}

	now := time.Now()
	if r.aptListen.lastImageAt.IsZero() || now.Sub(r.aptListen.lastImageAt) >= aptImageInterval {
		r.refreshAPTDecode()
		r.aptListen.lastImageAt = now
	}
	r.maybeEmitAPT()
}

func (r *Receiver) refreshAPTDecode() {
	audio := r.aptListen.audio
	if len(audio) < int(protocol.APTAudioRate) {
		return
	}
	lines := protocol.APTLineCount(audio, protocol.APTAudioRate)
	r.aptListen.lines = lines

	// Always attempt image synthesis from accumulated audio; line count alone
	// does not guarantee TryDecodePeak passes the 2400 Hz tone gate.
	if imgURL, imgLines, ok := protocol.DecodeAPTImage(audio, protocol.APTAudioRate); ok {
		r.aptListen.image = imgURL
		if imgLines > 0 {
			r.aptListen.lines = imgLines
		}
	}

	freqHz := r.aptListen.freqHz
	if freqHz == 0 {
		freqHz = aptCenterFreq
	}
	p := protocol.Peak{FreqHz: freqHz, PowerDB: float32(r.aptListen.strength)}
	band := protocol.Band{
		Name:     "气象卫星 APT",
		CenterHz: aptCenterFreq,
		RateHz:   aptSampleRate,
		Types:    []string{"satellite"},
	}
	dr, ok := protocol.TryDecodePeak(p, band, nil, aptCenterFreq, aptSampleRate, audio)
	if !ok {
		return
	}
	r.aptListen.decoded = true
	if dr.Decode != nil && dr.Decode.Metric != "" {
		r.aptListen.metric = dr.Decode.Metric
	}
	if dr.Image != "" {
		r.aptListen.image = dr.Image
	}
	if dr.Decode != nil && dr.Decode.ImageLines > 0 {
		r.aptListen.lines = dr.Decode.ImageLines
	}
}

func (r *Receiver) maybeEmitAPT() {
	if !r.aptListen.active {
		return
	}
	now := time.Now()
	if !r.aptListen.lastEmit.IsZero() && now.Sub(r.aptListen.lastEmit) < aptEmitInterval {
		return
	}
	r.aptListen.lastEmit = now

	lines := r.aptListen.lines
	if lines == 0 {
		lines = protocol.APTLineCount(r.aptListen.audio, protocol.APTAudioRate)
	}

	freqHz := r.aptListen.freqHz
	if freqHz == 0 {
		freqHz = aptCenterFreq
	}

	r.pushDecode(DecodeFrame{
		Service: ServiceAPT,
		APT: &APTPayload{
			FreqHz:     freqHz,
			Freq:       fmt.Sprintf("%.3f MHz", float64(freqHz)/1e6),
			Strength:   r.aptListen.strength,
			Lines:      lines,
			Image:      r.aptListen.image,
			Decoded:    r.aptListen.decoded,
			Metric:     r.aptListen.metric,
			Listening:  true,
			ElapsedSec: now.Sub(r.aptListen.startedAt).Seconds(),
		},
	})
}

func (r *Receiver) aptTuning() (center uint64, rate uint) {
	return aptCenterFreq, aptSampleRate
}

func (r *Receiver) aptDisplayConfig() Config {
	freqHz := r.aptListen.freqHz
	if freqHz == 0 {
		freqHz = aptCenterFreq
	}
	bw := r.config.FilterBW
	if bw < 20_000 || bw > 80_000 {
		bw = aptDefaultFilterBW
	}
	return Config{
		Service:    ServiceAPT,
		CenterFreq: aptCenterFreq,
		TuneFreq:   freqHz,
		SampleRate: aptSampleRate,
		FilterBW:   bw,
		Gain:       r.config.Gain,
		AGC:        r.config.AGC,
		Mode:       ModeFM,
	}
}
