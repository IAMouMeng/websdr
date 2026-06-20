package receiver

import (
	"time"

	"hz.tools/sdr"

	"github.com/iamoumeng/websdr/internal/adsb"
	"github.com/iamoumeng/websdr/internal/ais"
	"github.com/iamoumeng/websdr/internal/dsp"
	"github.com/iamoumeng/websdr/internal/protocol"
)

// DecodeFrame is one snapshot of a digital service's decoded state, pushed to
// clients roughly once a second while that service is active.
type DecodeFrame struct {
	Service          Service
	Aircraft         []adsb.AircraftState
	Vessels          []ais.VesselState
	Signals          []protocol.Signal
	ScanBand         string
	ScanProgress     *protocol.ScanProgress
	FullScanComplete bool
	BandSummaries    []protocol.BandSummary
	APT              *APTPayload
	LRPT             *LRPTPayload
	Meteor           *MeteorPayload
}

// aisChannel demodulates one of the two AIS channels: it mixes the channel down
// to baseband, low-pass filters and decimates to the per-channel working rate,
// then GMSK-demodulates and deframes.
type aisChannel struct {
	offset   float64 // channel offset from the tuner centre, Hz
	mixPhase float64
	filt     dsp.FIR
	demod    *ais.ChannelDemod
	buf      []complex128
}

const aisChannelRate = 48000.0

func (r *Receiver) initDecoders() {
	r.adsbDemod = adsb.NewDemodulator()
	r.adsbTracker = adsb.NewTracker()
	r.aisTracker = ais.NewTracker()

	mk := func(offset float64) *aisChannel {
		ch := &aisChannel{
			offset: offset,
			buf:    make([]complex128, 8192),
		}
		ch.demod = ais.NewChannelDemod(aisChannelRate, func(body []bool) {
			r.aisTracker.Update(ais.DecodePayload(body))
		})
		return ch
	}
	// 161.975 MHz and 162.025 MHz sit ±25 kHz from the 162.0 MHz tuner centre.
	r.aisChannels = []*aisChannel{mk(-25000), mk(+25000)}
}

// resetDecoders clears all decoder state. Must run on the read goroutine.
func (r *Receiver) resetDecoders() {
	r.adsbDemod.Reset()
	for _, ch := range r.aisChannels {
		ch.mixPhase = 0
		ch.filt.Reset()
		ch.demod.Reset()
	}
}

// effectiveTuning returns the hardware centre frequency and sample rate the
// active service requires. Radio uses the user-controlled values; the digital
// services use their fixed bands.
func effectiveTuning(cfg Config) (center uint64, rate uint) {
	return effectiveTuningAt(cfg, 0)
}

func effectiveTuningAt(cfg Config, protocolBandIdx int) (center uint64, rate uint) {
	switch cfg.Service {
	case ServiceADSB:
		return adsbCenterFreq, adsbSampleRate
	case ServiceAIS:
		return aisCenterFreq, aisSampleRate
	case ServiceProtocol:
		if protocolBandIdx >= 0 && protocolBandIdx < len(protocol.Bands) {
			b := protocol.Bands[protocolBandIdx]
			return b.CenterHz, b.RateHz
		}
		if len(protocol.Bands) > 0 {
			b := protocol.Bands[0]
			return b.CenterHz, b.RateHz
		}
		return cfg.CenterFreq, protocolSampleRate
	case ServiceAPT:
		return aptCenterFreq, aptSampleRate
	case ServiceLRPT:
		return lrptCenterFreq, lrptSampleRate
	case ServiceMeteor:
		return meteorDefaultCenter, meteorDefaultRate
	default:
		return cfg.CenterFreq, cfg.SampleRate
	}
}

// SetService switches what the tuner is doing, retuning the hardware and
// resetting all DSP/decoder state. The change is applied on the read goroutine.
func (r *Receiver) SetService(s Service) {
	if s != ServiceRadio && s != ServiceADSB && s != ServiceAIS && s != ServiceProtocol && s != ServiceAPT && s != ServiceLRPT && s != ServiceMeteor {
		return
	}
	r.mu.Lock()
	same := r.config.Service == s
	prev := r.config.Service
	r.config.Service = s
	if s == ServiceRadio && prev != ServiceRadio && prev != "" {
		r.config.CenterFreq = r.config.TuneFreq
		r.syncDirectSamplingLocked(r.config.TuneFreq)
	}
	r.mu.Unlock()

	r.enqueue(func() {
		r.applyServiceHardware(s, prev)
		if !same {
			r.resetDSP()
			r.resetDecoders()
			if s != ServiceAPT {
				r.aptListen.active = false
			}
			if s != ServiceLRPT {
				r.lrptListen.active = false
			}
			if s != ServiceMeteor {
				r.meteorListen.active = false
			}
		}
		r.lastDecodeEmit = time.Time{}
	})
}

func (r *Receiver) processADSB(samples sdr.SamplesU8) {
	r.feedADSB(samples)
	r.maybeEmitDecode(ServiceADSB)
}

func (r *Receiver) feedADSB(samples sdr.SamplesU8) {
	r.adsbDemod.Process(samples, func(msg []byte) {
		r.adsbTracker.Update(adsb.Decode(msg))
	})
}

func (r *Receiver) processAIS(iq []complex128, sr float64) {
	r.feedAIS(iq, sr)
	r.maybeEmitDecode(ServiceAIS)
}

func (r *Receiver) feedAIS(iq []complex128, sr float64) {
	factor := int(sr) / int(aisChannelRate)
	if factor < 1 {
		factor = 1
	}
	for _, ch := range r.aisChannels {
		work := r.workBuf[:len(iq)]
		copy(work, iq)
		dsp.MixDown(work, -ch.offset, sr, &ch.mixPhase)
		// 7 kHz one-sided cutoff: passes the ~±7 kHz GMSK channel and serves as
		// the anti-alias filter for the decimation to 48 kHz.
		n := ch.filt.DecimateComplex(work, ch.buf, factor, 7000, sr)
		ch.demod.Process(ch.buf[:n])
	}
}

// maybeEmitDecode pushes a fresh tracker snapshot at most once per second.
func (r *Receiver) maybeEmitDecode(s Service) {
	now := time.Now()
	if !r.lastDecodeEmit.IsZero() && now.Sub(r.lastDecodeEmit) < time.Second {
		return
	}
	r.lastDecodeEmit = now

	frame := DecodeFrame{Service: s}
	switch s {
	case ServiceADSB:
		frame.Aircraft = r.adsbTracker.Snapshot(60 * time.Second)
	case ServiceAIS:
		frame.Vessels = r.aisTracker.Snapshot(15 * time.Minute)
	}
	r.pushDecode(frame)
}

// pushDecode enqueues a decode snapshot, preferring the latest frame when the
// channel is backed up (large APT previews must not stall the read loop).
func (r *Receiver) pushDecode(frame DecodeFrame) {
	select {
	case r.decodeCh <- frame:
	default:
		select {
		case <-r.decodeCh:
		default:
		}
		select {
		case r.decodeCh <- frame:
		default:
		}
	}
}
