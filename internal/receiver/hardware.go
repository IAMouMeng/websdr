package receiver

import (
	"log"
	"time"

	"hz.tools/rf"
	"hz.tools/sdr/rtl"

	"github.com/iamoumeng/websdr/internal/rtlextra"
)

// reopenForReconfig stops IQ streaming, reopens the RTL-SDR with the current
// config, and restarts the read loops. Used when a hardware change cannot be
// applied to a live USB stream (radio ↔ digital band hops, HF direct sampling).
func (r *Receiver) reopenForReconfig(s Service, same bool) {
	r.streamMu.Lock()
	defer r.streamMu.Unlock()

	r.drainPendingCmds()

	if r.running.Load() {
		if err := r.stopRun(); err != nil {
			log.Printf("reopen stop: %v", err)
			return
		}
	} else if r.sdr != nil {
		_ = r.sdr.Close()
		r.sdr = nil
	}

	if !r.enabled.Load() {
		return
	}
	if err := r.startRun(); err != nil {
		log.Printf("reopen start: %v", err)
		return
	}
	r.enqueue(func() { r.postReopenReset(s, same) })
}

// postReopenReset clears DSP and, when the service changed, decoder/scan state
// after a device reopen. Runs on the read goroutine.
func (r *Receiver) postReopenReset(s Service, same bool) {
	r.resetDSP()
	if !same {
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
}

// forceNormalTunerModeIfNeeded disables HF direct sampling whenever the
// hardware is currently in it. The decision is keyed on appliedDirectMode (the
// real hardware state), NOT on r.config: callers such as SetDirectSampling(0)
// have already zeroed the config before getting here, so a config check would
// wrongly skip and leave the RTL2832 stuck in direct sampling. VHF-only users
// start with appliedDirectMode == DirectOff (see openDevice), so they still
// return early without touching the direct-sampling API.
func (r *Receiver) forceNormalTunerModeIfNeeded() {
	if r.sdr == nil || r.appliedDirectMode == rtlextra.DirectOff {
		return
	}
	if err := rtlextra.SetDirectSampling(r.sdr, rtlextra.DirectOff); err != nil {
		log.Printf("direct sampling: %v", err)
		return
	}
	r.appliedDirectMode = rtlextra.DirectOff
}

// applySDRMode applies the user's manual HF direct-sampling choice (radio only).
func (r *Receiver) applySDRMode(cfg Config) {
	if r.sdr == nil || cfg.Service != ServiceRadio || !directSamplingActive(cfg) {
		return
	}
	mode := cfg.DirectSampling
	if mode == r.appliedDirectMode {
		return
	}
	if err := rtlextra.SetDirectSampling(r.sdr, mode); err != nil {
		log.Printf("direct sampling: %v", err)
		return
	}
	r.appliedDirectMode = mode
}

// applyServiceHardware retunes the SDR for the active service.
func (r *Receiver) applyServiceHardware(s Service, prev Service) {
	if r.sdr == nil {
		return
	}
	switch s {
	case ServiceRadio:
		r.mu.Lock()
		if prev != ServiceRadio && prev != "" {
			r.config.CenterFreq = r.config.TuneFreq
			r.syncDirectSamplingLocked(r.config.TuneFreq)
		}
		cfg := r.config
		r.mu.Unlock()
		r.applySDRMode(cfg)
		if err := r.sdr.SetSampleRate(cfg.SampleRate); err != nil {
			log.Printf("service sample rate: %v", err)
		}
		if err := r.sdr.SetCenterFrequency(rf.Hz(cfg.CenterFreq)); err != nil {
			log.Printf("service frequency: %v", err)
		}
	default:
		r.forceNormalTunerModeIfNeeded()
		center, rate := effectiveTuning(r.Config())
		if s == ServiceProtocol {
			bands := r.protoScan.bands()
			idx := r.protoScan.bandIdx
			if idx < 0 || idx >= len(bands) {
				idx = 0
			}
			if len(bands) > 0 {
				center, rate = bands[idx].CenterHz, bands[idx].RateHz
			}
		}
		if err := r.sdr.SetSampleRate(rate); err != nil {
			log.Printf("service sample rate: %v", err)
		}
		if err := r.sdr.SetCenterFrequency(rf.Hz(center)); err != nil {
			log.Printf("service frequency: %v", err)
		}
	}
}

func (r *Receiver) applyRadioTuning(center uint64, rate uint) {
	if r.sdr == nil {
		return
	}
	if rate > 0 {
		if err := r.sdr.SetSampleRate(rate); err != nil {
			log.Printf("set sample rate: %v", err)
		}
	}
	if center > 0 {
		if err := r.sdr.SetCenterFrequency(rf.Hz(center)); err != nil {
			log.Printf("set frequency: %v", err)
		}
	}
}

// openDeviceExtras applies the service-aware HF direct-sampling mode on a
// freshly opened device. Always calls into librtlsdr — including DirectOff —
// because reopening the USB handle does not reset a prior Q/I session and
// leaving Q enabled during protocol/ADS-B retunes wedges the tuner (PLL never
// locks around the 24 MHz edge).
func openDeviceExtras(dev *rtl.Sdr, cfg Config) int {
	mode := desiredDirectMode(cfg)
	if err := rtlextra.SetDirectSampling(dev, mode); err != nil {
		log.Printf("direct sampling: %v", err)
	}
	return mode
}
