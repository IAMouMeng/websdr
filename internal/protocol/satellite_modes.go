package protocol

import (
	"fmt"
	"math"

	"github.com/iamoumeng/websdr/internal/dsp"
)

// VHF satellite downlink helpers (137 MHz).
var (
	lrptDownlinks = []uint64{
		137_900_000, // Meteor-M2 LRPT
		137_912_000, // Meteor-M2 LRPT (alt)
		137_825_000, // Meteor-M N2
	}
	dsbDownlinks = []uint64{
		137_350_000, // NOAA TIP / DSB
		137_770_000,
	}
	hrptDownlinks = []uint64{
		1_698_000_000,
		1_702_500_000,
		1_707_000_000,
	}
)

const (
	lrptSnapHz   = uint64(100_000)
	dsbSnapHz    = uint64(60_000)
	hrptSnapHz   = uint64(5_000_000)
	lrptMinBwHz  = 60_000
	lrptMaxBwHz  = 350_000
	dsbMinBwHz   = 15_000
	dsbMaxBwHz   = 55_000
	hrptMinBwHz  = 300_000
)

func SnapLRPTDownlink(freqHz uint64) uint64  { return snapNearest(freqHz, lrptDownlinks, lrptSnapHz) }
func SnapDSBDownlink(freqHz uint64) uint64   { return snapNearest(freqHz, dsbDownlinks, dsbSnapHz) }
func SnapHRPTDownlink(freqHz uint64) uint64  { return snapNearest(freqHz, hrptDownlinks, hrptSnapHz) }

func snapNearest(freqHz uint64, channels []uint64, maxSnap uint64) uint64 {
	best := freqHz
	bestDist := maxSnap + 1
	for _, ch := range channels {
		var dist uint64
		if freqHz >= ch {
			dist = freqHz - ch
		} else {
			dist = ch - freqHz
		}
		if dist < bestDist {
			bestDist = dist
			best = ch
		}
	}
	if bestDist <= maxSnap {
		return best
	}
	return freqHz
}

func isLRPTFreq(freqHz uint64) bool {
	return distNearest(freqHz, lrptDownlinks) <= lrptSnapHz
}

func isDSBFreq(freqHz uint64) bool {
	return distNearest(freqHz, dsbDownlinks) <= dsbSnapHz
}

func isHRPTFreq(freqHz uint64) bool {
	return freqHz >= 1_697_000_000 && freqHz <= 1_710_000_000
}

func distNearest(freqHz uint64, channels []uint64) uint64 {
	best := uint64(^uint64(0) >> 1)
	for _, ch := range channels {
		var d uint64
		if freqHz >= ch {
			d = freqHz - ch
		} else {
			d = ch - freqHz
		}
		if d < best {
			best = d
		}
	}
	return best
}

// TrySatelliteVHF picks DSB/TIP, LRPT, or APT on the 137 MHz weather band.
func TrySatelliteVHF(peak Peak, iq []complex128, sr float64, centerHz uint64, dwellAudio []float32) (DecodeResult, bool) {
	if isDSBFreq(peak.FreqHz) && peak.BwHz >= dsbMinBwHz && peak.BwHz <= dsbMaxBwHz {
		if dr, ok := tryDSB(peak, iq, sr, centerHz); ok {
			return dr, true
		}
	}
	if peak.BwHz >= lrptMinBwHz && peak.BwHz <= lrptMaxBwHz {
		if dr, ok := tryLRPT(peak, iq, sr, centerHz); ok {
			return dr, true
		}
	}
	if dr, ok := tryAPT(peak, iq, sr, centerHz, peak.FreqHz, dwellAudio); ok {
		return dr, true
	}
	return DecodeResult{}, false
}

func tryLRPT(peak Peak, iq []complex128, sr float64, centerHz uint64) (DecodeResult, bool) {
	freqHz := SnapLRPTDownlink(peak.FreqHz)
	if !isLRPTFreq(freqHz) {
		return DecodeResult{}, false
	}
	offsetHz := float64(freqHz) - float64(centerHz)
	if fm := FMAudioFromIQ(iq, sr, offsetHz); len(fm) >= 2048 {
		if toneRatio(fm, APTAudioRate, 2400) >= 0.12 {
			return DecodeResult{}, false
		}
	}
	symRate := estimateSymbolRate(iq, sr, offsetHz)
	metric := "数字载波"
	if symRate >= 50_000 && symRate <= 120_000 {
		metric = fmt.Sprintf("%.0f sym/s", symRate)
	}
	fMHz := float64(freqHz) / 1e6
	return DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("LRPT %.3f MHz", fMHz),
		Mod:     "OQPSK",
		Service: "Meteor LRPT",
		Cols: map[string]interface{}{
			"svc": "LRPT 已确认", "dir": "卫星下行", "mod": "OQPSK", "sub": metric,
		},
		Decode: &DecodeInfo{
			Service:     "Meteor LRPT",
			Mod:         "OQPSK",
			Direction:   "卫星下行",
			Metric:      metric,
			MetricLabel: "符号率估计",
			Note:        "进入 LRPT 页监听数字链路；完整图像需后续帧解码",
		},
		Details: [][2]string{
			{"频率", fmt.Sprintf("%.3f MHz", fMHz)},
			{"调制", "OQPSK (Meteor LRPT)"},
			{"估计带宽", fmt.Sprintf("%.0f kHz", peak.BwHz/1000)},
			{"符号率", metric},
			{"检测", "数字载波已确认"},
		},
	}, true
}

func tryDSB(peak Peak, iq []complex128, sr float64, centerHz uint64) (DecodeResult, bool) {
	freqHz := SnapDSBDownlink(peak.FreqHz)
	if !isDSBFreq(freqHz) {
		return DecodeResult{}, false
	}
	if peak.BwHz < dsbMinBwHz || peak.BwHz > dsbMaxBwHz {
		return DecodeResult{}, false
	}
	offsetHz := float64(freqHz) - float64(centerHz)
	if fm := FMAudioFromIQ(iq, sr, offsetHz); len(fm) >= 2048 {
		r2400 := toneRatio(fm, APTAudioRate, 2400)
		if r2400 >= 0.15 {
			return DecodeResult{}, false
		}
	}
	fMHz := float64(freqHz) / 1e6
	return DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("DSB/TIP %.3f MHz", fMHz),
		Mod:     "PSK",
		Service: "NOAA DSB/TIP",
		Cols: map[string]interface{}{
			"svc": "DSB/TIP", "dir": "卫星下行", "mod": "PSK", "sub": "8320 bps",
		},
		Decode: &DecodeInfo{
			Service:     "NOAA DSB/TIP",
			Mod:         "Split-phase PSK",
			Direction:   "卫星下行",
			Metric:      "8320 bps",
			MetricLabel: "遥测速率",
			Note:        "NOAA 星载 TIP 遥测；可用 USB 模式收听，多数卫星已停发",
		},
		Details: [][2]string{
			{"频率", fmt.Sprintf("%.3f MHz", fMHz)},
			{"调制", "Direct Sounder Broadcast (PSK)"},
			{"速率", "8320 bps Manchester"},
			{"估计带宽", fmt.Sprintf("%.0f kHz", peak.BwHz/1000)},
			{"检测", "遥测载波已确认"},
		},
	}, true
}

// TryHRPT identifies an L-band HRPT digital downlink (carrier only).
func TryHRPT(peak Peak) (DecodeResult, bool) {
	if !isHRPTFreq(peak.FreqHz) || peak.BwHz < hrptMinBwHz {
		return DecodeResult{}, false
	}
	freqHz := SnapHRPTDownlink(peak.FreqHz)
	fMHz := float64(freqHz) / 1e6
	return DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("HRPT %.3f MHz", fMHz),
		Mod:     "QPSK",
		Service: "NOAA HRPT",
		Cols: map[string]interface{}{
			"svc": "HRPT", "dir": "卫星下行", "mod": "QPSK", "sub": "~665 kbps",
		},
		Decode: &DecodeInfo{
			Service:     "NOAA HRPT",
			Mod:         "QPSK",
			Direction:   "卫星下行",
			Metric:      "~665 kbps",
			MetricLabel: "数据速率",
			Note:        "L 波段高分辨率云图；需 1.7 GHz 天线，进入无线电页 RAW/USB 收听",
		},
		Details: [][2]string{
			{"频率", fmt.Sprintf("%.3f MHz", fMHz)},
			{"调制", "HRPT QPSK"},
			{"速率", "~665 kbps"},
			{"估计带宽", fmt.Sprintf("%.1f MHz", peak.BwHz/1e6)},
			{"检测", "数字载波已确认（图像解码未实现）"},
		},
	}, true
}

// EstimateSymbolRatePublic estimates line/symbol rate from IQ (exported for receiver).
func EstimateSymbolRatePublic(iq []complex128, sr, offsetHz float64) float64 {
	return estimateSymbolRate(iq, sr, offsetHz)
}

func estimateSymbolRate(iq []complex128, sr, offsetHz float64) float64 {
	if len(iq) < 4096 {
		return 0
	}
	work := make([]complex128, len(iq))
	copy(work, iq)
	var phase float64
	dsp.MixDown(work, offsetHz, sr, &phase)
	n := len(work)
	if n > 16384 {
		work = work[:16384]
		n = len(work)
	}
	mag := make([]float32, n)
	for i, v := range work {
		mag[i] = float32(math.Hypot(real(v), imag(v)))
	}
	// Remove DC
	var mean float64
	for _, v := range mag {
		mean += float64(v)
	}
	mean /= float64(n)
	for i := range mag {
		mag[i] -= float32(mean)
	}
	bestFreq, bestP := 0.0, 0.0
	for _, target := range []float64{72000, 80000, 8320} {
		p := toneRatio(mag, sr, target)
		if p > bestP {
			bestP = p
			bestFreq = target
		}
	}
	if bestP < 0.02 {
		return 0
	}
	return bestFreq
}
