package protocol

import (
	"fmt"
	"strings"
)

// morseTable ITU Morse code (subset used for validation).
var morseTable = map[string]string{
	".-": "A", "-...": "B", "-.-.": "C", "-..": "D", ".": "E",
	"..-.": "F", "--.": "G", "....": "H", "..": "I", ".---": "J",
	"-.-": "K", ".-..": "L", "--": "M", "-.": "N", "---": "O",
	".--.": "P", "--.-": "Q", ".-.": "R", "...": "S", "-": "T",
	"..-": "U", "...-": "V", ".--": "W", "-..-": "X", "-.--": "Y",
	"--..": "Z", ".----": "1", "..---": "2", "...--": "3", "....-": "4",
	".....": "5", "-....": "6", "--...": "7", "---..": "8", "----.": "9",
	"-----": "0",
}

// TryCWFromWaterfall analyzes a time–frequency waterfall slice for OOK keying
// and attempts Morse decode to confirm CW.
func TryCWFromWaterfall(peak Peak, band Band, spectra [][]float32, centerHz uint64, rateHz uint) (DecodeResult, bool) {
	if !IsCWPeak(peak) || !IsCWDedicatedBand(band) || len(spectra) < 20 {
		return DecodeResult{}, false
	}
	nBins := len(spectra[0])
	if nBins == 0 {
		return DecodeResult{}, false
	}

	series := extractPowerSeries(spectra, peak.FreqHz, centerHz, rateHz, nBins)
	if len(series) < 10 {
		return DecodeResult{}, false
	}

	crossings := powerCrossings(series)
	if crossings < 3 {
		return DecodeResult{}, false
	}

	text, wpm, ok := decodeMorseFromPower(series)
	if !ok || len(text) < 1 {
		return DecodeResult{}, false
	}

	fMHz := float64(peak.FreqHz) / 1e6
	seg := cwSegmentLabel(peak.FreqHz)
	note := fmt.Sprintf("瀑布键控 · 摩尔斯 \"%s\"", text)
	if wpm > 0 {
		note = fmt.Sprintf("瀑布键控 · %d WPM · \"%s\"", wpm, text)
	}

	return DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("CW %.3f MHz", fMHz),
		Mod:     "CW",
		Service: "业余 CW",
		Cols: map[string]interface{}{
			"mod": "CW", "band": seg, "note": text, "fmt": fmt.Sprintf("%d WPM", wpm),
		},
		Decode: &DecodeInfo{
			Service:     "业余 CW / 摩尔斯",
			Mod:         "CW",
			Metric:      text,
			MetricLabel: "解码",
			Note:        note,
		},
		Details: [][2]string{
			{"频率", fmt.Sprintf("%.3f MHz", fMHz)},
			{"调制", "CW (OOK)"},
			{"业余波段", seg},
			{"摩尔斯", text},
			{"速率", fmt.Sprintf("%d WPM", wpm)},
			{"检测", "瀑布断续键控 + 摩尔斯解析"},
		},
	}, true
}

func extractPowerSeries(spectra [][]float32, freqHz, centerHz uint64, rateHz uint, nBins int) []float32 {
	bin := freqToBin(freqHz, centerHz, rateHz, nBins)
	out := make([]float32, len(spectra))
	for i, row := range spectra {
		if bin < len(row) {
			out[i] = row[bin]
		}
	}
	return out
}

func freqToBin(freqHz, centerHz uint64, rateHz uint, nBins int) int {
	frac := float64(freqHz)/float64(rateHz) - float64(centerHz)/float64(rateHz) + 0.5
	b := int(frac * float64(nBins))
	if b < 0 {
		return 0
	}
	if b >= nBins {
		return nBins - 1
	}
	return b
}

func powerCrossings(power []float32) int {
	if len(power) < 4 {
		return 0
	}
	p10, p90 := percentiles(power, 10, 90)
	thr := p10 + 0.35*(p90-p10)
	above := power[0] > thr
	n := 0
	for i := 1; i < len(power); i++ {
		now := power[i] > thr
		if now != above {
			n++
			above = now
		}
	}
	return n
}

func decodeMorseFromPower(power []float32) (text string, wpm int, ok bool) {
	if len(power) < 12 {
		return "", 0, false
	}
	p10, p90 := percentiles(power, 15, 85)
	thr := p10 + 0.38*(p90-p10)

	on := make([]bool, len(power))
	for i, v := range power {
		on[i] = v > thr
	}

	type run struct {
		on  bool
		len int
	}
	var runs []run
	cur := on[0]
	curLen := 1
	for i := 1; i < len(on); i++ {
		if on[i] == cur {
			curLen++
		} else {
			runs = append(runs, run{cur, curLen})
			cur = on[i]
			curLen = 1
		}
	}
	runs = append(runs, run{cur, curLen})

	unit := 0
	for _, r := range runs {
		if r.on && r.len > 0 && (unit == 0 || r.len < unit) {
			unit = r.len
		}
	}
	if unit < 1 {
		return "", 0, false
	}
	// All ON runs same length and >= 3× shortest guess → treat as dah-only, unit = len/3.
	allOnSame := true
	for _, r := range runs {
		if !r.on {
			continue
		}
		if r.len != unit {
			allOnSame = false
			break
		}
	}
	if allOnSame && unit >= 3 {
		unit = (unit + 1) / 3
		if unit < 1 {
			unit = 1
		}
	}

	var curSym strings.Builder
	var decoded strings.Builder
	valid := 0

	flushSym := func() {
		if curSym.Len() == 0 {
			return
		}
		if ch, found := morseTable[curSym.String()]; found {
			decoded.WriteString(ch)
			valid++
		}
		curSym.Reset()
	}

	for _, r := range runs {
		units := (r.len + unit/2) / unit
		if r.on {
			if units >= 3 {
				curSym.WriteByte('-')
			} else {
				curSym.WriteByte('.')
			}
			continue
		}
		if units >= 7 {
			flushSym()
		} else if units >= 3 {
			flushSym()
		}
	}
	flushSym()

	if valid < 1 {
		return "", 0, false
	}

	totalRows := len(power)
	estWPM := 18
	if totalRows > 0 && unit > 0 {
		secPerRow := 3.0 / float64(totalRows)
		ditSec := float64(unit) * secPerRow
		if ditSec > 0 {
			estWPM = int(60.0 / (50.0 * ditSec))
		}
	}
	if estWPM < 5 || estWPM > 60 {
		estWPM = 18
	}
	return decoded.String(), estWPM, true
}
