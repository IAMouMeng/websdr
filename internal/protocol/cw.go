package protocol

import (
	"fmt"
	"math"
	"sort"

	"github.com/iamoumeng/websdr/internal/dsp"
)

const (
	cwMaxBwHz     = 1200 // real CW is usually < 500 Hz in spectrum
	cwMinBwHz     = 80
	cwMinPowerDB  = float32(-84)
	cwAudioSR     = 8000.0
	cwEnvSmooth   = 40 // ~5 ms at 8 kHz
	cwMinCrest    = 0.12
	cwMinKeyPower = 0.05
	cwMinCrossHz  = 2.5
	cwMaxCrossHz  = 55.0
)

// ITU-style CW sub-bands inside amateur allocations.
var cwSegments = []struct {
	loMHz float64
	hiMHz float64
	label string
}{
	{24.890, 24.990, "12m CW"},
	{28.000, 28.500, "10m CW"},
	{50.000, 50.150, "6m CW"},
	{144.000, 144.250, "2m CW"},
	{432.000, 432.125, "70cm CW"},
}

// IsCWDedicatedBand is true when a scan stop overlaps a known CW segment.
func IsCWDedicatedBand(band Band) bool {
	return NeedsCWAnalysis(band)
}

// NeedsCWAnalysis reports whether a dwell should linger for envelope keying.
func NeedsCWAnalysis(band Band) bool {
	if !HasType(band, "cw") || band.Decode != "" {
		return false
	}
	lo := int64(band.CenterHz) - int64(band.RateHz)/2
	hi := int64(band.CenterHz) + int64(band.RateHz)/2
	for _, s := range cwSegments {
		slo := int64(s.loMHz * 1e6)
		shi := int64(s.hiMHz * 1e6)
		if hi >= slo && lo <= shi {
			return true
		}
	}
	return false
}

// IsInCWSegment reports whether freq lies in a known CW activity segment.
func IsInCWSegment(freqHz uint64) bool {
	fMHz := float64(freqHz) / 1e6
	for _, s := range cwSegments {
		if fMHz >= s.loMHz && fMHz <= s.hiMHz {
			return true
		}
	}
	return false
}

func cwSegmentLabel(freqHz uint64) string {
	fMHz := float64(freqHz) / 1e6
	for _, s := range cwSegments {
		if fMHz >= s.loMHz && fMHz <= s.hiMHz {
			return s.label
		}
	}
	return amateurBandLabel(fMHz)
}

// IsCWPeak reports whether a spectrum peak might be CW (classification hint only).
func IsCWPeak(peak Peak) bool {
	if peak.BwHz < cwMinBwHz || peak.BwHz > cwMaxBwHz {
		return false
	}
	if peak.PowerDB < cwMinPowerDB {
		return false
	}
	if !IsInCWSegment(peak.FreqHz) {
		return false
	}
	return IsAmateurFreq(peak.FreqHz)
}

// IsAmateurFreq returns true for common amateur allocations reachable by RTL-SDR.
func IsAmateurFreq(freqHz uint64) bool {
	fMHz := float64(freqHz) / 1e6
	switch {
	case fMHz >= 28.0 && fMHz <= 29.7:
		return true
	case fMHz >= 50.0 && fMHz <= 54.0:
		return true
	case fMHz >= 144.0 && fMHz <= 148.0:
		return true
	case fMHz >= 420.0 && fMHz <= 450.0:
		if fMHz >= 433.05 && fMHz <= 434.79 {
			return false
		}
		return true
	default:
		return false
	}
}

type cwMetrics struct {
	Crest      float64
	KeyPower   float64
	CrossHz    float64
	RejectNote string
}

// TryCWPeak confirms CW using waterfall Morse decode, then envelope keying fallback.
func TryCWPeak(peak Peak, band Band, iq []complex128, sr float64, centerHz uint64, spectra [][]float32) (DecodeResult, bool) {
	if !IsCWPeak(peak) || !IsCWDedicatedBand(band) {
		return DecodeResult{}, false
	}
	if len(spectra) >= 20 {
		if dr, ok := TryCWFromWaterfall(peak, band, spectra, centerHz, uint(sr)); ok {
			return dr, true
		}
	}
	if len(iq) < 4096 {
		return DecodeResult{}, false
	}

	offsetHz := float64(peak.FreqHz) - float64(centerHz)
	env := cwEnvelope(iq, sr, offsetHz)
	if len(env) < 384 {
		return DecodeResult{}, false
	}
	if cwLooksLikeFM(iq, sr, offsetHz) {
		return DecodeResult{}, false
	}

	m := analyzeCWKeying(env, cwAudioSR)
	if !m.ok() {
		return DecodeResult{}, false
	}

	fMHz := float64(peak.FreqHz) / 1e6
	bwLabel := formatBwHz(peak.BwHz)
	seg := cwSegmentLabel(peak.FreqHz)
	note := "包络键控已确认"
	if m.CrossHz > 0 {
		note = fmt.Sprintf("键控 %.1f 次/秒", m.CrossHz)
	}

	return DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("CW %.3f MHz", fMHz),
		Mod:     "CW",
		Service: "业余 CW",
		Cols: map[string]interface{}{
			"mod": "CW", "bw": bwLabel, "band": seg, "note": note,
		},
		Decode: &DecodeInfo{
			Service:     "业余 CW / 摩尔斯",
			Mod:         "CW",
			Metric:      fmt.Sprintf("%.0f%%", m.KeyPower*100),
			MetricLabel: "键控频段能量",
			Note:        "点击右侧图标进入无线电页 CW 模式收听",
		},
		Details: [][2]string{
			{"频率", fmt.Sprintf("%.3f MHz", fMHz)},
			{"调制", "CW (OOK 键控)"},
			{"估计带宽", bwLabel},
			{"业余波段", seg},
			{"包络起伏", fmt.Sprintf("%.0f%%", m.Crest*100)},
			{"键控速率", fmt.Sprintf("%.1f /s", m.CrossHz)},
			{"检测", "键控特征已确认"},
		},
	}, true
}

func formatBwHz(bw float64) string {
	if bw >= 1000 {
		return fmt.Sprintf("%.1f kHz", bw/1000)
	}
	return fmt.Sprintf("%.0f Hz", bw)
}

func (m cwMetrics) ok() bool {
	return m.Crest >= cwMinCrest &&
		m.KeyPower >= cwMinKeyPower &&
		m.CrossHz >= cwMinCrossHz &&
		m.CrossHz <= cwMaxCrossHz
}

func cwEnvelope(iq []complex128, sr, offsetHz float64) []float32 {
	work := make([]complex128, len(iq))
	copy(work, iq)
	var phase float64
	dsp.MixDown(work, offsetHz, sr, &phase)

	factor := int(sr / cwAudioSR)
	if factor < 1 {
		factor = 1
	}
	n := len(work) / factor
	if n < 256 {
		return nil
	}
	dec := make([]complex128, n)
	for i := 0; i < n; i++ {
		dec[i] = work[i*factor]
	}

	env := make([]float32, n)
	dsp.AMDemod(dec, env)
	smoothMovingAvg(env, cwEnvSmooth)
	return env
}

func analyzeCWKeying(env []float32, sr float64) cwMetrics {
	m := cwMetrics{}
	if len(env) < 512 {
		return m
	}

	work := make([]float32, len(env))
	copy(work, env)
	mean := float32(0)
	for _, v := range work {
		mean += v
	}
	mean /= float32(len(work))
	if mean < 1e-9 {
		return m
	}
	for i := range work {
		work[i] -= mean
	}

	p10, p90 := percentiles(env, 10, 90)
	m.Crest = float64(p90-p10) / float64(mean)
	m.KeyPower = bandPowerRatio(work, sr, 3, 28)
	m.CrossHz = envelopeCrossings(env, sr)
	return m
}

func cwLooksLikeFM(iq []complex128, sr, offsetHz float64) bool {
	work := make([]complex128, len(iq))
	copy(work, iq)
	var phase float64
	dsp.MixDown(work, offsetHz, sr, &phase)

	factor := int(sr / cwAudioSR)
	if factor < 1 {
		factor = 1
	}
	n := len(work) / factor
	if n < 512 {
		return false
	}
	dec := make([]complex128, n)
	for i := 0; i < n; i++ {
		dec[i] = work[i*factor]
	}
	workSR := sr / float64(factor)

	fm := make([]float32, n)
	var prev complex128
	dsp.FMDemod(dec, 400, workSR, &prev, fm)

	fmRMS := audioRMS(fm)
	env := make([]float32, n)
	dsp.AMDemod(dec, env)
	smoothMovingAvg(env, cwEnvSmooth)
	p10, p90 := percentiles(env, 10, 90)
	var mean float32
	for _, v := range env {
		mean += v
	}
	mean /= float32(len(env))
	crest := 0.0
	if mean > 1e-9 {
		crest = float64(p90-p10) / float64(mean)
	}
	// FM spur / birdie: steady envelope but discriminator still produces audio.
	return crest < 0.12 && fmRMS > 0.02
}

func audioRMS(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	if len(x) == 0 {
		return 0
	}
	return math.Sqrt(s / float64(len(x)))
}

func smoothMovingAvg(x []float32, win int) {
	if win < 2 || len(x) == 0 {
		return
	}
	tmp := append([]float32(nil), x...)
	half := win / 2
	for i := range x {
		var sum float64
		c := 0
		for j := i - half; j <= i+half; j++ {
			if j >= 0 && j < len(tmp) {
				sum += float64(tmp[j])
				c++
			}
		}
		x[i] = float32(sum / float64(c))
	}
}

func percentiles(x []float32, p10, p90 int) (float32, float32) {
	cp := append([]float32(nil), x...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	i10 := len(cp) * p10 / 100
	i90 := len(cp) * p90 / 100
	if i10 >= len(cp) {
		i10 = len(cp) - 1
	}
	if i90 >= len(cp) {
		i90 = len(cp) - 1
	}
	return cp[i10], cp[i90]
}

func bandPowerRatio(x []float32, sr, loHz, hiHz float64) float64 {
	total := 0.0
	for _, v := range x {
		total += float64(v) * float64(v)
	}
	if total < 1e-12 {
		return 0
	}
	best := 0.0
	for f := loHz; f <= hiHz; f += 1.0 {
		if p := goertzelPower(x, sr, f); p > best {
			best = p
		}
	}
	return best / total
}

func envelopeCrossings(env []float32, sr float64) float64 {
	if len(env) < 32 {
		return 0
	}
	p10, p90 := percentiles(env, 10, 90)
	thr := p10 + 0.38*(p90-p10)
	above := env[0] > thr
	crosses := 0
	for i := 1; i < len(env); i++ {
		now := env[i] > thr
		if now != above {
			crosses++
			above = now
		}
	}
	dur := float64(len(env)) / sr
	if dur <= 0 {
		return 0
	}
	return float64(crosses) / dur / 2
}

func amateurBandLabel(fMHz float64) string {
	switch {
	case fMHz >= 28.0 && fMHz <= 29.7:
		return "10m HF"
	case fMHz >= 50.0 && fMHz <= 54.0:
		return "6m VHF"
	case fMHz >= 144.0 && fMHz <= 148.0:
		return "2m VHF"
	case fMHz >= 420.0 && fMHz <= 450.0:
		return "70cm UHF"
	default:
		return "业余"
	}
}
