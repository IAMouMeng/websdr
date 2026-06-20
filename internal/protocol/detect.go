package protocol

import (
	"fmt"
	"math"
	"sort"
)

const (
	peakThresholdDB = 5.0
	minPeakBins     = 2
	maxPeaks        = 24
)

// DetectPeaks finds carriers in an averaged power spectrum (dB, len FFTSize/2).
// Bin i maps to center + (i/len - 0.5) * sampleRate.
func DetectPeaks(db []float32, centerHz uint64, sampleRate uint) []Peak {
	if len(db) == 0 {
		return nil
	}

	floor := noiseFloor(db)
	var raw []struct {
		bin int
		db  float32
	}
	for i, v := range db {
		if float32(v)-floor >= peakThresholdDB {
			raw = append(raw, struct {
				bin int
				db  float32
			}{i, v})
		}
	}
	if len(raw) == 0 {
		return nil
	}

	sort.Slice(raw, func(i, j int) bool { return raw[i].db > raw[j].db })

	used := make([]bool, len(db))
	var peaks []Peak
	for _, r := range raw {
		if used[r.bin] {
			continue
		}
		lo, hi := r.bin, r.bin
		sum := float64(r.db)
		n := 1
		used[r.bin] = true
		for lo > 0 && float32(db[lo-1])-floor >= peakThresholdDB*0.5 {
			lo--
			if !used[lo] {
				used[lo] = true
				sum += float64(db[lo])
				n++
			}
		}
		for hi+1 < len(db) && float32(db[hi+1])-floor >= peakThresholdDB*0.5 {
			hi++
			if !used[hi] {
				used[hi] = true
				sum += float64(db[hi])
				n++
			}
		}
		if hi-lo+1 < minPeakBins && n < minPeakBins {
			continue
		}
		centerBin := (lo + hi) / 2
		freq := binToFreq(centerHz, sampleRate, centerBin, len(db))
		bw := float64(hi-lo+1) * float64(sampleRate) / float64(len(db)*2)
		peaks = append(peaks, Peak{
			FreqHz:  freq,
			PowerDB: float32(sum / float64(n)),
			BwHz:    bw,
		})
		if len(peaks) >= maxPeaks {
			break
		}
	}
	return peaks
}

func noiseFloor(db []float32) float32 {
	cp := append([]float32(nil), db...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := len(cp) * 3 / 10
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func binToFreq(centerHz uint64, sampleRate uint, bin, nBins int) uint64 {
	frac := float64(bin)/float64(nBins) - 0.5
	offset := frac * float64(sampleRate)
	f := float64(centerHz) + offset
	if f < 0 {
		f = 0
	}
	return uint64(f + 0.5)
}

// Classify picks a UI protocol type and builds a display row from a peak.
func Classify(peak Peak, band Band) Signal {
	typ := pickType(peak, band)
	freqMHz := float64(peak.FreqHz) / 1e6
	strength := int(math.Round(float64(peak.PowerDB)))
	if strength > -20 {
		strength = -20
	}
	if strength < -115 {
		strength = -115
	}

	id := fmt.Sprintf("%s-%d", typ, peak.FreqHz/1000)
	label := fmt.Sprintf("%.3f MHz", freqMHz)
	freqStr := label

	sig := Signal{
		ID:          id,
		Type:        typ,
		Label:       label,
		Freq:        freqStr,
		FreqHz:      peak.FreqHz,
		Strength:    strength,
		StrengthKey: "rssi",
		Cols:        map[string]interface{}{},
		Details:     nil,
	}

	switch typ {
	case "broadcast":
		sig.Label = fmt.Sprintf("疑似 FM %.1f MHz", freqMHz)
		sig.Cols = map[string]interface{}{
			"pi": "—", "ps": "—", "pty": "—", "af": fmt.Sprintf("%.1f", freqMHz),
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波，解调确认中"},
			{"扫描段", band.Name},
		}
	case "acars":
		sig.Label = fmt.Sprintf("ACARS %.3f MHz", freqMHz)
		sig.Cols = map[string]interface{}{
			"flight": "—", "reg": "—", "mode": "VHF", "label": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"调制", "AM-MSK (预期)"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
	case "paging":
		sig.Cols = map[string]interface{}{
			"proto": "POCSAG/FLEX", "baud": "—", "capcode": "—", "func": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"调制", "FSK (预期)"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
	case "aprs":
		sig.Cols = map[string]interface{}{
			"call": "—", "fmt": "APRS", "path": "—", "type": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"格式", "1200 AFSK (预期)"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
	case "cw":
		seg := cwSegmentLabel(peak.FreqHz)
		note := "CW 段载波"
		if IsCWDedicatedBand(band) {
			note = "待键控确认"
		}
		sig.Label = fmt.Sprintf("CW? %s", freqStr)
		sig.Cols = map[string]interface{}{
			"mod": "CW?", "bw": formatBwHz(peak.BwHz), "band": seg, "note": note,
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"调制", "CW? (等幅电报)"},
			{"估计带宽", formatBwHz(peak.BwHz)},
			{"业余波段", seg},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", note},
			{"扫描段", band.Name},
		}
	case "dmr":
		sig.Cols = map[string]interface{}{
			"mode": "DMR/P25", "cc": "—", "tg": "—", "slot": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"估计带宽", fmt.Sprintf("%.0f kHz", peak.BwHz/1000)},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
	case "tetra":
		sig.Cols = map[string]interface{}{
			"mcc": "—", "mnc": "—", "la": "—", "carrier": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"制式", "TETRA (预期)"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
	case "lora":
		sig.Cols = map[string]interface{}{
			"sf": "—", "bw": "—", "cr": "—", "sync": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"调制", "LoRa CSS (疑似)"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
	case "ism":
		sig.Cols = map[string]interface{}{
			"band": bandLabel(peak.FreqHz), "mod": "OOK/FSK", "proto": "—", "rate": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"类型", "ISM 突发"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"扫描段", band.Name},
		}
		case "satellite":
		sig.Label = fmt.Sprintf("疑似卫星 %s", freqStr)
		kind := "APT/LRPT/DSB?"
		if freqMHz >= 1690 && freqMHz <= 1715 {
			kind = "HRPT?"
		} else if freqMHz >= 137.82 && freqMHz <= 137.95 {
			kind = "LRPT?"
		} else if (freqMHz >= 137.30 && freqMHz <= 137.40) || (freqMHz >= 137.72 && freqMHz <= 137.82) {
			kind = "DSB/TIP?"
		} else if freqMHz >= 136 && freqMHz <= 138 {
			if IsNearAPTDownlink(peak.FreqHz) {
				kind = "APT?"
			} else {
				kind = "VHF 载波?"
			}
		} else if freqMHz >= 144 && freqMHz <= 147 {
			if _, name := SnapAmateurSat(peak.FreqHz); name != "" {
				kind = name + "?"
			} else {
				kind = "业余卫星?"
			}
		} else if freqMHz >= 435 && freqMHz <= 438 {
			if _, name := SnapAmateurSat(peak.FreqHz); name != "" {
				kind = name + "?"
			} else {
				kind = "业余卫星?"
			}
		}
		sig.Cols = map[string]interface{}{
			"svc": kind, "dir": "—", "mod": "—", "pass": "—",
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"业务", kind + "（频段推断）"},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波，解调确认中"},
			{"扫描段", band.Name},
		}
	case "cellular":
		sig.StrengthKey = "rsrp"
		info := InferCellular(peak.FreqHz, peak.BwHz)
		sig.Label = fmt.Sprintf("%s %s", info.Band, freqStr)
		if info.Band == "—" {
			sig.Label = fmt.Sprintf("蜂窝 %s", freqStr)
		}
		sig.Cols = map[string]interface{}{
			"rat": info.RAT, "band": info.Band, "pci": info.PCI,
			"earfcn": info.ARFCN, "bw": info.Bw,
		}
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"制式", info.RAT},
			{"频段", info.Band},
			{"ARFCN / EARFCN", info.ARFCN},
			{"PCI / 小区", info.PCI},
			{"信道带宽", info.Bw},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"检测", "频谱载波"},
			{"说明", info.Note},
			{"扫描段", band.Name},
		}
	default:
		sig.Details = [][2]string{
			{"频率", freqStr},
			{"强度", fmt.Sprintf("%d dBm", strength)},
			{"扫描段", band.Name},
		}
	}
	return sig
}

func pickType(peak Peak, band Band) string {
	bwK := peak.BwHz / 1000
	fMHz := float64(peak.FreqHz) / 1e6

	if HasType(band, "satellite") && IsNearAmateurSat(peak.FreqHz) &&
		peak.BwHz >= amsatMinBwHz && peak.BwHz <= amsatMaxBwHz {
		return "satellite"
	}

	for _, typ := range band.Types {
		switch typ {
		case "cw":
			if IsCWPeak(peak) {
				return "cw"
			}
		case "broadcast":
			if fMHz >= 87.5 && fMHz <= 108 && bwK >= 80 {
				return "broadcast"
			}
		case "acars":
			if fMHz >= 130 && fMHz <= 137 {
				return "acars"
			}
		case "aprs":
			if fMHz >= 144 && fMHz <= 146 && bwK < 30 {
				return "aprs"
			}
		case "satellite":
			if fMHz >= 136 && fMHz <= 138 {
				return "satellite"
			}
			if fMHz >= 144 && fMHz <= 147 {
				return "satellite"
			}
			if fMHz >= 435 && fMHz <= 438 {
				return "satellite"
			}
		case "paging":
			if fMHz >= 147 && fMHz <= 153 && bwK < 30 {
				return "paging"
			}
		case "dmr":
			if bwK >= 6 && bwK <= 25 {
				return "dmr"
			}
		case "tetra":
			if bwK >= 10 && bwK <= 40 {
				return "tetra"
			}
		case "lora":
			if bwK >= 100 && bwK <= 600 {
				return "lora"
			}
		case "ism":
			if bwK < 80 {
				return "ism"
			}
		case "cellular":
			if fMHz >= 880 && fMHz <= 960 {
				return "cellular"
			}
			if fMHz >= 1710 && fMHz <= 1920 {
				return "cellular"
			}
			if fMHz >= 1805 && fMHz <= 1880 {
				return "cellular"
			}
		}
	}
	if len(band.Types) > 0 {
		return band.Types[0]
	}
	return "ism"
}

func bandLabel(freqHz uint64) string {
	mhz := float64(freqHz) / 1e6
	switch {
	case mhz < 400:
		return "VHF"
	case mhz < 500:
		return "433 MHz"
	default:
		return "868 MHz"
	}
}
