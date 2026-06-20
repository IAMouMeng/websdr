package receiver

import (
	"fmt"
	"time"

	"hz.tools/rf"
	"hz.tools/sdr"

	"github.com/iamoumeng/websdr/internal/dsp"
	"github.com/iamoumeng/websdr/internal/protocol"
	"github.com/iamoumeng/websdr/internal/satellite"
)

const (
	meteorDefaultCenter = 137_900_000
	meteorDefaultRate   = 2_048_000
	meteorDefaultFilter = 150_000
	meteorEmitInterval  = 400 * time.Millisecond
	meteorUnlockStreak  = 15 // keep demod alive through brief lock dropouts
)

// MeteorChannelState is one MSU-MR channel decode status.
type MeteorChannelState struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Band    string `json:"band"`
	Active  bool   `json:"active"`
	Lines   int    `json:"lines,omitempty"`
	Image   string `json:"image,omitempty"`
}

// MeteorPayload is the satellite decode page snapshot pushed over WebSocket.
type MeteorPayload struct {
	FreqHz        uint64               `json:"freqHz"`
	NominalFreqHz uint64               `json:"nominalFreqHz"`
	CenterFreqHz  uint64               `json:"centerFreqHz"`
	SampleRateHz  uint                 `json:"sampleRateHz"`
	DopplerHz       float64              `json:"dopplerHz"`
	ManualOffsetHz  float64              `json:"manualOffsetHz"`
	Freq            string               `json:"freq"`
	Strength      int                  `json:"strength"`
	Metric        string               `json:"metric,omitempty"`
	Locked        bool                 `json:"locked"`
	Synced        bool                 `json:"synced"`
	Decoded       bool                 `json:"decoded"`
	Lines         int                  `json:"lines"`
	Image         string               `json:"image,omitempty"`
	Listening     bool                 `json:"listening"`
	ElapsedSec    float64              `json:"elapsedSec"`
	Elevation     float64              `json:"elevation"`
	Azimuth       float64              `json:"azimuth"`
	AutoDoppler   bool                 `json:"autoDoppler"`
	Norad         int                  `json:"norad"`
	Satellite     string               `json:"satellite"`
	Constellation []float64            `json:"constellation,omitempty"`
	Channels      []MeteorChannelState `json:"channels"`
}

type meteorListenState struct {
	active        bool
	nominalFreqHz uint64
	centerFreqHz  uint64
	sampleRateHz  uint
	dopplerHz      float64
	manualOffsetHz float64
	offsetHz       float64
	norad         int
	satName       string
	modulation    string
	symRate       float64
	strength      int
	metric        string
	locked        bool
	unlockStreak  int
	holdComposite string
	holdChannels  [6]string
	holdLines     int
	lastImageLines int
	holdSynced    bool
	autoDoppler   bool
	elevation     float64
	azimuth       float64
	startedAt     time.Time
	lastEmit      time.Time
}

func (r *Receiver) initMeteorListen() {
	r.meteorListen = meteorListenState{}
}

func (r *Receiver) SetMeteorListen(on bool, freqHz uint64, norad int, autoDoppler bool) {
	r.enqueue(func() {
		if on {
			entry := satellite.Lookup(norad)
			if entry == nil {
				entry = &satellite.SatelliteCatalog[0]
				norad = entry.Norad
			}
			if entry.Modulation == "LRPT" {
				if freqHz < 136_000_000 || freqHz > 138_500_000 {
					freqHz = entry.Downlink
				}
				freqHz = protocol.SnapLRPTDownlink(freqHz)
			} else {
				if freqHz < 1_600_000_000 || freqHz > 1_750_000_000 {
					freqHz = entry.Downlink
				}
			}
			center := entry.CenterHz
			rate := entry.SampleRate
			if center == 0 {
				center = meteorDefaultCenter
			}
			if rate == 0 {
				rate = meteorDefaultRate
			}
			r.meteorListen.active = true
			r.meteorListen.nominalFreqHz = freqHz
			r.meteorListen.centerFreqHz = center
			r.meteorListen.sampleRateHz = rate
			r.meteorListen.dopplerHz = 0
			r.meteorListen.manualOffsetHz = 0
			r.meteorListen.norad = norad
			r.meteorListen.satName = entry.Name
			r.meteorListen.modulation = entry.Modulation
			r.meteorListen.symRate = entry.SymbolRate
			r.meteorListen.autoDoppler = autoDoppler
			r.meteorListen.metric = ""
			r.meteorListen.locked = false
			r.meteorListen.unlockStreak = 0
			r.meteorListen.holdComposite = ""
			r.meteorListen.holdChannels = [6]string{}
			r.meteorListen.holdLines = 0
			r.meteorListen.lastImageLines = 0
			r.meteorListen.holdSynced = false
			r.meteorListen.startedAt = time.Now()
			r.meteorListen.lastEmit = time.Time{}
			if entry.Modulation != "LRIT" {
				if r.meteorDemod == nil {
					r.meteorDemod = protocol.NewMeteorDemod(entry.SymbolRate)
				} else {
					r.meteorDemod.Reset(entry.SymbolRate)
				}
			}
			r.applyMeteorTune()
		} else {
			r.meteorListen.active = false
		}
	})
}

// SetMeteorTrack updates pass geometry and optional Doppler from the client.
func (r *Receiver) SetMeteorTrack(dopplerHz, elevation, azimuth float64) {
	r.enqueue(func() {
		if !r.meteorListen.active {
			return
		}
		r.meteorListen.elevation = elevation
		r.meteorListen.azimuth = azimuth
		if r.meteorListen.autoDoppler {
			prev := r.meteorListen.dopplerHz
			r.meteorListen.dopplerHz = dopplerHz
			if int64(prev) != int64(dopplerHz) {
				r.applyMeteorTune()
			}
		}
	})
}

// SetMeteorManualTune sets the absolute receive frequency; auto Doppler still applies
// and manualOffsetHz stores the fine correction on top.
func (r *Receiver) SetMeteorManualTune(freqHz uint64) {
	r.enqueue(func() {
		if !r.meteorListen.active || freqHz == 0 {
			return
		}
		entry := satellite.Lookup(r.meteorListen.norad)
		if entry != nil && entry.Modulation == "LRPT" {
			freqHz = protocol.SnapLRPTDownlink(freqHz)
		}
		base := int64(r.meteorListen.nominalFreqHz)
		if r.meteorListen.autoDoppler {
			base += int64(r.meteorListen.dopplerHz)
		}
		r.meteorListen.manualOffsetHz = float64(int64(freqHz) - base)
		r.applyMeteorTune()
	})
}

func (r *Receiver) meteorTuneHz() uint64 {
	tune := int64(r.meteorListen.nominalFreqHz)
	if r.meteorListen.autoDoppler {
		tune += int64(r.meteorListen.dopplerHz)
	}
	tune += int64(r.meteorListen.manualOffsetHz)
	if tune < 0 {
		return 0
	}
	return uint64(tune)
}

func (r *Receiver) applyMeteorTune() {
	tuneHz := r.meteorTuneHz()
	center := r.meteorListen.centerFreqHz
	if center == 0 {
		center = meteorDefaultCenter
	}
	rate := r.meteorListen.sampleRateHz
	if rate == 0 {
		rate = meteorDefaultRate
	}
	r.meteorListen.offsetHz = float64(tuneHz) - float64(center)
	if r.sdr != nil {
		r.forceNormalTunerModeIfNeeded()
		_ = r.sdr.SetSampleRate(rate)
		_ = r.sdr.SetCenterFrequency(rf.Hz(center))
	}
}

func (r *Receiver) processMeteor(samples sdr.SamplesU8) {
	if !r.meteorListen.active {
		return
	}
	if r.meteorRecord.active && !r.meteorRecord.paused {
		r.pushMeteorIQ(samples)
	}
	n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
	if n < dsp.FFTSize {
		r.maybeEmitMeteor(nil, protocol.MeteorDemodResult{})
		return
	}
	iq := r.iqBuf[:n]
	cfg := r.meteorDisplayConfig()
	r.emitSpectrum(iq, cfg)
	sampleRate := r.meteorListen.sampleRateHz
	if sampleRate == 0 {
		sampleRate = meteorDefaultRate
	}
	r.processAudio(iq, cfg, float64(sampleRate))

	center := r.meteorListen.centerFreqHz
	if center == 0 {
		center = meteorDefaultCenter
	}
	entry := satellite.Lookup(r.meteorListen.norad)

	dsp.PowerSpectrum(iq, spectrumSegs, r.fftBuf, r.window, r.powBuf, r.specOut)
	peak := protocol.DetectPeaks(r.specOut, center, sampleRate)
	if len(peak) > 0 {
		r.meteorListen.strength = int(peak[0].PowerDB)
	}

	offset := r.meteorListen.offsetHz

	// GK-2A / LRIT: spectrum + constellation preview only — never run LRPT line demod.
	if entry != nil && entry.Modulation == "LRIT" {
		r.meteorListen.locked = len(peak) > 0 && r.meteorListen.strength > -55
		if r.meteorListen.locked {
			r.meteorListen.metric = "LRIT 载波"
		} else if r.meteorListen.metric == "" || r.meteorListen.metric == "LRIT 载波" {
			r.meteorListen.metric = "LRIT 搜锁"
		}
		lritSR := entry.SymbolRate
		if lritSR <= 0 {
			lritSR = 2_000_000
		}
		constellation := protocol.OQPSKConstellation(iq, float64(sampleRate), offset, lritSR, 128)
		r.maybeEmitMeteor(constellation, protocol.MeteorDemodResult{})
		return
	}

	symRate := protocol.EstimateSymbolRatePublic(iq, float64(sampleRate), offset)
	if satellite.SymbolRateLocked(entry, symRate) {
		r.meteorListen.unlockStreak = 0
		r.meteorListen.locked = true
		r.meteorListen.symRate = symRate
		r.meteorListen.metric = fmt.Sprintf("%.0f sym/s", symRate)
	} else if r.meteorListen.locked {
		r.meteorListen.unlockStreak++
		if r.meteorListen.unlockStreak >= meteorUnlockStreak {
			r.meteorListen.locked = false
			if symRate > 0 {
				r.meteorListen.metric = fmt.Sprintf("%.0f sym/s?", symRate)
			}
		}
	} else {
		if symRate > 0 {
			r.meteorListen.metric = fmt.Sprintf("%.0f sym/s?", symRate)
		}
	}

	var constellation []float64
	var dec protocol.MeteorDemodResult
	sr := r.meteorListen.symRate
	if sr <= 0 && entry != nil {
		sr = entry.SymbolRate
	}
	if sr <= 0 {
		sr = 72000
	}
	demodActive := r.meteorListen.locked || (r.meteorListen.unlockStreak > 0 && r.meteorListen.unlockStreak < meteorUnlockStreak)
	if demodActive {
		if r.meteorDemod == nil {
			r.meteorDemod = protocol.NewMeteorDemod(sr)
		}
		dec = r.meteorDemod.Process(iq, float64(sampleRate), offset)
		constellation = dec.Constellation
		if len(constellation) == 0 {
			constellation = protocol.OQPSKConstellation(iq, float64(sampleRate), offset, sr, 256)
		}
	}
	r.maybeEmitMeteor(constellation, dec)
}

func (r *Receiver) maybeEmitMeteor(constellation []float64, dec protocol.MeteorDemodResult) {
	if !r.meteorListen.active {
		return
	}
	now := time.Now()
	if !r.meteorListen.lastEmit.IsZero() && now.Sub(r.meteorListen.lastEmit) < meteorEmitInterval {
		return
	}
	r.meteorListen.lastEmit = now

	tuneHz := r.meteorTuneHz()

	entry := satellite.Lookup(r.meteorListen.norad)
	chDefs := satellite.ChannelsFor(entry)

	// Only refresh image buffers while carrier-locked and new scan lines arrived.
	if r.meteorListen.locked && dec.Lines > r.meteorListen.lastImageLines {
		if dec.Composite != "" {
			r.meteorListen.holdComposite = dec.Composite
		}
		for i := 0; i < 6; i++ {
			if dec.ChannelImages[i] != "" {
				r.meteorListen.holdChannels[i] = dec.ChannelImages[i]
			}
		}
		r.meteorListen.lastImageLines = dec.Lines
		r.meteorListen.holdLines = dec.Lines
	}
	if dec.Synced {
		r.meteorListen.holdSynced = true
	}

	channels := make([]MeteorChannelState, len(chDefs))
	linesOut := r.meteorListen.holdLines
	for i, ch := range chDefs {
		img := ""
		if i < len(r.meteorListen.holdChannels) {
			img = r.meteorListen.holdChannels[i]
		}
		channels[i] = MeteorChannelState{
			ID: ch.ID, Name: ch.Name, Band: ch.Band,
			Active: linesOut >= 2, Lines: linesOut, Image: img,
		}
	}

	r.pushDecode(DecodeFrame{
		Service: ServiceMeteor,
		Meteor: &MeteorPayload{
			FreqHz:        tuneHz,
			NominalFreqHz: r.meteorListen.nominalFreqHz,
			CenterFreqHz:  r.meteorListen.centerFreqHz,
			SampleRateHz:  r.meteorListen.sampleRateHz,
			DopplerHz:        r.meteorListen.dopplerHz,
			ManualOffsetHz:   r.meteorListen.manualOffsetHz,
			Freq:             fmt.Sprintf("%.3f MHz", float64(tuneHz)/1e6),
			Strength:      r.meteorListen.strength,
			Metric:        r.meteorListen.metric,
			Locked:        r.meteorListen.locked,
			Synced:        r.meteorListen.holdSynced,
			Decoded:       r.meteorListen.holdSynced && linesOut >= 12,
			Lines:         linesOut,
			Image:         r.meteorListen.holdComposite,
			Listening:     true,
			ElapsedSec:    now.Sub(r.meteorListen.startedAt).Seconds(),
			Elevation:     r.meteorListen.elevation,
			Azimuth:       r.meteorListen.azimuth,
			AutoDoppler:   r.meteorListen.autoDoppler,
			Norad:         r.meteorListen.norad,
			Satellite:     r.meteorListen.satName,
			Constellation: constellation,
			Channels:      channels,
		},
	})
}

func (r *Receiver) meteorDisplayConfig() Config {
	tuneHz := r.meteorTuneHz()
	center := r.meteorListen.centerFreqHz
	if center == 0 {
		center = meteorDefaultCenter
	}
	rate := r.meteorListen.sampleRateHz
	if rate == 0 {
		rate = meteorDefaultRate
	}
	bw := r.config.FilterBW
	if entry := satellite.Lookup(r.meteorListen.norad); entry != nil && entry.Modulation == "LRIT" {
		if bw < 200_000 || bw > 2_000_000 {
			bw = 800_000
		}
	} else if bw < 80_000 || bw > 250_000 {
		bw = meteorDefaultFilter
	}
	return Config{
		Service:    ServiceMeteor,
		CenterFreq: center,
		TuneFreq:   tuneHz,
		SampleRate: rate,
		FilterBW:   bw,
		Mode:       ModeUSB,
	}
}

func (r *Receiver) meteorTuning() (center uint64, rate uint) {
	center = r.meteorListen.centerFreqHz
	if center == 0 {
		center = meteorDefaultCenter
	}
	rate = r.meteorListen.sampleRateHz
	if rate == 0 {
		rate = meteorDefaultRate
	}
	return center, rate
}
