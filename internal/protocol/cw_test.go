package protocol

import (
	"math"
	"testing"
)

func TestIsCWPeak(t *testing.T) {
	if !IsCWPeak(Peak{FreqHz: 144_100_000, PowerDB: -70, BwHz: 500}) {
		t.Fatal("expected 2m CW segment narrow peak")
	}
	if IsCWPeak(Peak{FreqHz: 145_500_000, PowerDB: -70, BwHz: 500}) {
		t.Fatal("FM segment of 2m should not be CW")
	}
	if IsCWPeak(Peak{FreqHz: 144_100_000, PowerDB: -70, BwHz: 8000}) {
		t.Fatal("wide peak should not be CW")
	}
	if IsCWPeak(Peak{FreqHz: 433_920_000, PowerDB: -70, BwHz: 400}) {
		t.Fatal("ISM freq should not be CW")
	}
}

func TestIsInCWSegment(t *testing.T) {
	if !IsInCWSegment(28_150_000) {
		t.Fatal("28.15 should be 10m CW")
	}
	if !IsInCWSegment(24_940_000) {
		t.Fatal("24.94 should be 12m CW")
	}
	if IsInCWSegment(28_800_000) {
		t.Fatal("28.8 is phone segment")
	}
}

func TestAnalyzeCWKeyingSynthetic(t *testing.T) {
	sr := cwAudioSR
	n := int(sr * 0.35)
	env := synthCWEnvelope(n, sr, 18)
	m := analyzeCWKeying(env, sr)
	if !m.ok() {
		t.Fatalf("synthetic CW should pass: crest=%.2f key=%.3f cross=%.1f", m.Crest, m.KeyPower, m.CrossHz)
	}
}

func TestAnalyzeCWKeyingSteadyCarrier(t *testing.T) {
	sr := cwAudioSR
	n := int(sr * 0.35)
	env := make([]float32, n)
	for i := range env {
		env[i] = 1.0
	}
	m := analyzeCWKeying(env, sr)
	if m.ok() {
		t.Fatal("steady carrier should not pass as keyed CW")
	}
}

func TestTryCWPeakRequiresDedicatedBand(t *testing.T) {
	band2m := Band{Name: "业余 2m", CenterHz: 145_825_000, RateHz: 1_024_000, Types: []string{"cw", "satellite", "aprs"}}
	iq := synthCWIQ(131072, 2_048_000, 20)
	peak := Peak{FreqHz: 145_500_000, PowerDB: -60, BwHz: 400}
	if _, ok := TryCWPeak(peak, band2m, iq, 2_048_000, 145_825_000, nil); ok {
		t.Fatal("2m FM segment should not confirm CW")
	}

	band10m := Band{Name: "10米 CW (28 MHz)", CenterHz: 28_300_000, RateHz: 2_048_000, Types: []string{"cw"}}
	peak = Peak{FreqHz: 28_150_000, PowerDB: -60, BwHz: 400}
	if _, ok := TryCWPeak(peak, band10m, iq, 2_048_000, 28_300_000, nil); !ok {
		t.Fatal("10m CW segment with keyed IQ should confirm CW")
	}
}

func synthCWEnvelope(n int, sr float64, wpm float64) []float32 {
	dit := sr * 60 / (50 * wpm)
	env := make([]float32, n)
	for i := range env {
		pos := float64(i)
		on := math.Mod(pos, dit*2) < dit
		if on {
			env[i] = 1.0
		} else {
			env[i] = 0.12
		}
	}
	smoothMovingAvg(env, cwEnvSmooth)
	return env
}

func synthCWIQ(n int, sr float64, wpm float64) []complex128 {
	env := synthCWEnvelope(n, cwAudioSR, wpm)
	factor := int(sr / cwAudioSR)
	if factor < 1 {
		factor = 1
	}
	iq := make([]complex128, n)
	for i := range iq {
		e := env[i/factor]
		phase := 2 * math.Pi * 0.31 * float64(i)
		amp := 0.4 * float64(e)
		iq[i] = complex(amp*math.Cos(phase), amp*math.Sin(phase))
	}
	return iq
}
