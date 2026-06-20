package receiver

import "testing"

func TestSnapFreqForMode(t *testing.T) {
	normal := Config{DirectSampling: 0}
	hf := Config{DirectSampling: 2}

	if got := snapFreqForMode(7_030_000, normal); got != normalMinHz {
		t.Fatalf("HF on normal mode clamps to min VHF: got %d", got)
	}
	if got := snapFreqForMode(7_030_000, hf); got != 7_030_000 {
		t.Fatalf("7.030 MHz on HF mode: got %d", got)
	}
}

func TestDirectSamplingActive(t *testing.T) {
	if directSamplingActive(Config{DirectSampling: 0}) {
		t.Fatal("off should be inactive")
	}
	if !directSamplingActive(Config{DirectSampling: 2}) {
		t.Fatal("Q channel should be active")
	}
}

func TestDirectSamplingForFreq(t *testing.T) {
	if got := directSamplingForFreq(7_030_000); got != 2 {
		t.Fatalf("7.030 MHz: got mode %d want Q(2)", got)
	}
	if got := directSamplingForFreq(100_000_000); got != 0 {
		t.Fatalf("100 MHz: got mode %d want off", got)
	}
}

func TestSyncDirectSamplingLocked(t *testing.T) {
	r := &Receiver{config: Config{TuneFreq: 100_000_000, SampleRate: 2_048_000}}
	r.mu.Lock()
	if !r.syncDirectSamplingLocked(7_030_000) {
		t.Fatal("expected mode change to HF")
	}
	if r.config.DirectSampling != 2 {
		t.Fatalf("want Q channel, got %d", r.config.DirectSampling)
	}
	if r.config.SampleRate != 1_024_000 {
		t.Fatalf("sample rate capped: got %d", r.config.SampleRate)
	}
	r.mu.Unlock()
}

func TestCrossesRadioBand(t *testing.T) {
	if crossesRadioBand(ServiceRadio, ServiceRadio) {
		t.Fatal("same service")
	}
	if !crossesRadioBand(ServiceRadio, ServiceADSB) {
		t.Fatal("radio -> adsb")
	}
	if !crossesRadioBand(ServiceADSB, ServiceRadio) {
		t.Fatal("adsb -> radio")
	}
	if crossesRadioBand(ServiceADSB, ServiceAIS) {
		t.Fatal("digital -> digital should not reopen")
	}
}
