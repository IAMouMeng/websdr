package receiver

import "github.com/iamoumeng/websdr/internal/rtlextra"

// RTL-SDR tuning limits (Hz).
const (
	normalMinHz = 24_000_000
	normalMaxHz = 1_766_000_000
	hfMinHz     = 100_000    // avoid DC spike
	hfMaxHz     = 28_800_000 // RTL2832 xtal
)

func directSamplingActive(cfg Config) bool {
	return cfg.DirectSampling == rtlextra.DirectI || cfg.DirectSampling == rtlextra.DirectQ
}

// directSamplingForFreq picks HF Q-channel direct sampling below 24 MHz.
func directSamplingForFreq(freq uint64) int {
	if freq < normalMinHz {
		return rtlextra.DirectQ
	}
	return rtlextra.DirectOff
}

// syncDirectSamplingLocked updates DirectSampling from the listen frequency.
// Caller must hold r.mu. Returns true when the mode changed.
func (r *Receiver) syncDirectSamplingLocked(freq uint64) bool {
	want := directSamplingForFreq(freq)
	if r.config.DirectSampling == want {
		return false
	}
	r.config.DirectSampling = want
	if directSamplingActive(r.config) {
		if r.config.SampleRate > 1_024_000 {
			r.config.SampleRate = 1_024_000
		}
	} else if r.config.SampleRate < 1_500_000 {
		r.config.SampleRate = 2_048_000
	}
	return true
}

// desiredDirectMode is the HF direct-sampling mode the hardware should be in for
// this config: the user's choice while on the radio service, and off for every
// digital service (ADS-B/AIS/… use the normal R820T tuner). The radio direct-
// sampling preference is kept in config across digital switches, so this — not
// config.DirectSampling alone — is the real target hardware state.
func desiredDirectMode(cfg Config) int {
	if cfg.Service == ServiceRadio && directSamplingActive(cfg) {
		return cfg.DirectSampling
	}
	return rtlextra.DirectOff
}

// crossesRadioBand reports whether switching between prev and s moves between
// the user-tuned radio band and a fixed digital-service band. Large jumps like
// 100 MHz ↔ 1090 MHz cannot be applied reliably on a live USB stream.
func crossesRadioBand(prev, s Service) bool {
	if prev == "" || prev == s {
		return false
	}
	return prev == ServiceRadio || s == ServiceRadio
}

// freqHopNeedsReopen reports whether retuning between two centers on a live USB
// stream is unsafe (same threshold as protocol band hops).
func freqHopNeedsReopen(a, b uint64) bool {
	if a == 0 || b == 0 {
		return false
	}
	d := int64(a) - int64(b)
	if d < 0 {
		d = -d
	}
	return uint64(d) > 40_000_000
}

func snapFreqForMode(hz uint64, cfg Config) uint64 {
	if directSamplingActive(cfg) {
		if hz < hfMinHz {
			return hfMinHz
		}
		if hz > hfMaxHz {
			return hfMaxHz
		}
		return hz
	}
	if hz < normalMinHz {
		return normalMinHz
	}
	if hz > normalMaxHz {
		return normalMaxHz
	}
	return hz
}
