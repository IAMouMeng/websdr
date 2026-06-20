package protocol

import (
	"fmt"
	"math"

	"github.com/iamoumeng/websdr/internal/dsp"
)

// DecodeResult is produced when inline demod confirms a signal type.
type DecodeResult struct {
	OK      bool
	Label   string
	Mod     string
	Service string
	Cols    map[string]interface{}
	Details [][2]string
	Decode  *DecodeInfo
	Image   string
}

// TryDecodePeak attempts FM/WFM demod on dwell IQ for one detected carrier.
// dwellAudio is optional accumulated FM audio during the band dwell (for APT images).
func TryDecodePeak(peak Peak, band Band, iq []complex128, centerHz uint64, sampleRate float64, dwellAudio []float32) (DecodeResult, bool) {
	if len(iq) < 4096 && len(dwellAudio) < 4096 {
		return DecodeResult{}, false
	}
	offset := float64(peak.FreqHz) - float64(centerHz)
	fMHz := float64(peak.FreqHz) / 1e6

	for _, typ := range band.Types {
		switch typ {
		case "cw":
			if dr, ok := TryCWPeak(peak, band, iq, sampleRate, centerHz, nil); ok {
				return dr, true
			}
		case "satellite":
			if fMHz >= 144 && fMHz <= 147 || fMHz >= 435 && fMHz <= 438 {
				if dr, ok := TryAmateurSatellite(peak); ok {
					return dr, true
				}
			}
			if fMHz >= 136 && fMHz <= 138 {
				if dr, ok := TrySatelliteVHF(peak, iq, sampleRate, centerHz, dwellAudio); ok {
					return dr, true
				}
			}
			if fMHz >= 1690 && fMHz <= 1715 {
				if dr, ok := TryHRPT(peak); ok {
					return dr, true
				}
			}
		case "broadcast":
			if fMHz >= 87.5 && fMHz <= 108 {
				if dr, ok := tryFMBroadcast(iq, sampleRate, offset, peak.FreqHz); ok {
					return dr, true
				}
			}
		}
	}
	return DecodeResult{}, false
}

// MergeDecode overlays demod results onto a classified carrier row.
func MergeDecode(sig Signal, dr DecodeResult) Signal {
	if !dr.OK {
		return sig
	}
	sig.Decoded = true
	if dr.Label != "" {
		sig.Label = dr.Label
	}
	if dr.Cols != nil {
		if sig.Cols == nil {
			sig.Cols = map[string]interface{}{}
		}
		for k, v := range dr.Cols {
			sig.Cols[k] = v
		}
	}
	if len(dr.Details) > 0 {
		sig.Details = dr.Details
	}
	if dr.Decode != nil {
		sig.Decode = dr.Decode
	}
	if dr.Image != "" {
		sig.Image = dr.Image
	}
	return sig
}

func tryAPT(peak Peak, iq []complex128, sr float64, centerHz, freqHz uint64, dwellAudio []float32) (DecodeResult, bool) {
	if !IsNearAPTDownlink(freqHz) {
		return DecodeResult{}, false
	}
	freqHz = SnapAPTDownlink(freqHz)
	if peak.BwHz > 0 && (peak.BwHz < aptMinBwHz || peak.BwHz > aptMaxBwHz) {
		return DecodeResult{}, false
	}
	if peak.PowerDB < aptMinPowerDB {
		return DecodeResult{}, false
	}
	offsetHz := float64(freqHz) - float64(centerHz)

	fmAudio := dwellAudio
	if len(fmAudio) < int(APTAudioRate) {
		fmAudio = FMAudioFromIQ(iq, sr, offsetHz)
	}
	if len(fmAudio) < 2048 {
		return DecodeResult{}, false
	}
	workSR := APTAudioRate

	r2400 := toneRatio(fmAudio, workSR, 2400)
	r2080 := toneRatio(fmAudio, workSR, 2080)
	r800 := toneRatio(fmAudio, workSR, 800)
	r2400c := math.Min(r2400, 1.0)
	r2080c := math.Min(r2080, 1.0)

	// Strong dual-subcarrier signature; short scan IQ often fakes one tone.
	if r2400c < 0.28 || r2080c < 0.10 || r2400 < r800*4.0 {
		return DecodeResult{}, false
	}
	if r2080 < r2400*0.35 || r2080 > r2400*2.8 {
		return DecodeResult{}, false
	}

	imgURL, imgLines, imgOK := "", 0, false
	if len(fmAudio) >= int(APTAudioRate*0.6) {
		imgURL, imgLines, imgOK = DecodeAPTImage(fmAudio, workSR)
	}

	// Protocol scan keeps <0.2 s IQ; without a real APT image this is noise.
	scanClip := len(dwellAudio) == 0 && len(fmAudio) < int(APTAudioRate*2)
	if scanClip && !imgOK {
		return DecodeResult{}, false
	}
	if !imgOK && (r2400c < 0.40 || r2080c < 0.14) {
		return DecodeResult{}, false
	}

	svcLabel := "APT 已确认"
	detectNote := "图像已解码"
	if !imgOK {
		svcLabel = "APT 疑似"
		detectNote = "副载波匹配（待图像确认）"
	}

	dr := DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("APT %.3f MHz", float64(freqHz)/1e6),
		Mod:     "FM",
		Service: "NOAA APT",
		Cols: map[string]interface{}{
			"svc": svcLabel, "dir": "卫星下行", "mod": "FM", "pass": "—",
			"sub": fmt.Sprintf("%.1f%%", r2400c*100),
		},
		Decode: &DecodeInfo{
			Service:     "NOAA APT",
			Mod:         "FM",
			Direction:   "卫星下行",
			Subcarrier:  "2400 Hz 可见光 / 2080 Hz 红外",
			Metric:      fmt.Sprintf("%.1f%%", r2400c*100),
			MetricLabel: "2400 Hz 能量占比",
			Note:        "进入 APT 页长时间接收；多数 NOAA 星 APT 已停发",
		},
		Details: [][2]string{
			{"频率", fmt.Sprintf("%.3f MHz", float64(freqHz)/1e6)},
			{"调制", "FM"},
			{"副载波", "2400 / 2080 Hz"},
			{"2400 Hz 占比", fmt.Sprintf("%.1f%%", r2400c*100)},
			{"2080 Hz 占比", fmt.Sprintf("%.1f%%", r2080c*100)},
			{"检测", detectNote},
		},
	}

	if imgOK {
		dr.Image = imgURL
		if imgLines > 0 {
			dr.Decode.ImageLines = imgLines
			dr.Cols["pass"] = fmt.Sprintf("%d 行", imgLines)
			dr.Details = append(dr.Details, [2]string{"图像行数", fmt.Sprintf("%d 行", imgLines)})
		}
	} else if linesEst := APTLineCount(fmAudio, workSR); linesEst > 0 {
		dr.Decode.ImageLines = linesEst
		dr.Cols["pass"] = fmt.Sprintf("%d 行?", linesEst)
	}

	return dr, true
}

func tryFMBroadcast(iq []complex128, sr, offsetHz float64, freqHz uint64) (DecodeResult, bool) {
	work := make([]complex128, len(iq))
	copy(work, iq)
	var phase float64
	dsp.MixDown(work, offsetHz, sr, &phase)

	factor := int(sr / 384_000)
	if factor < 1 {
		factor = 1
	}
	dec := make([]complex128, len(work)/factor+1)
	n := decimate(work, dec, factor)
	if n < 2048 {
		return DecodeResult{}, false
	}
	workSR := sr / float64(factor)

	audio := make([]float32, n)
	var fmPrev complex128
	dsp.FMDemod(dec[:n], 75_000, workSR, &fmPrev, audio)
	dsp.DeemphasisFM(audio[:n], workSR, new(float32))

	r19 := toneRatio(audio, workSR, 19_000)
	r57 := toneRatio(audio, workSR, 57_000)
	if r19 < 0.015 && r57 < 0.008 {
		return DecodeResult{}, false
	}

	freqMHz := float64(freqHz) / 1e6
	stereo := "单声道"
	if r19 >= 0.015 {
		stereo = "立体声导频"
	}
	rdsNote := "57 kHz 副载波已检测"
	piCol, psCol, ptyCol := "—", "—", stereo
	if r57 >= 0.008 {
		rdsNote = "RDS 副载波"
		if rds, ok := DecodeRDS(audio, workSR); ok {
			if rds.PI != "" {
				piCol = rds.PI
			}
			if rds.PS != "" && rds.PS != "—" {
				psCol = rds.PS
				rdsNote = "RDS 已解码"
			}
			if rds.PTY != "" {
				ptyCol = rds.PTY
			}
		}
	}

	label := fmt.Sprintf("FM %.1f MHz", freqMHz)
	if psCol != "—" {
		label = fmt.Sprintf("FM %s %.1f", psCol, freqMHz)
	}

	decode := &DecodeInfo{
		Service:     "FM 广播",
		Mod:         "WFM",
		Subcarrier:  rdsNote,
		Metric:      fmt.Sprintf("导频 %.2f%%", r19*100),
		MetricLabel: "19 kHz 立体声导频",
		PI:          piCol,
		PS:          psCol,
		PTY:         ptyCol,
	}
	if piCol == "—" && psCol == "—" {
		decode.Note = "RDS 台名/PI 需更长监听或更强信号"
	}

	details := [][2]string{
		{"频率", fmt.Sprintf("%.3f MHz", freqMHz)},
		{"调制", "WFM 广播 FM"},
		{"19 kHz 导频", fmt.Sprintf("%.2f%%", r19*100)},
		{"57 kHz RDS", rdsNote},
		{"检测", "已解调确认"},
	}
	if piCol != "—" {
		details = append(details, [2]string{"PI Code", piCol})
	}
	if psCol != "—" {
		details = append(details, [2]string{"PS 台名", psCol})
	}
	if ptyCol != stereo && ptyCol != "—" {
		details = append(details, [2]string{"PTY", ptyCol})
	} else {
		details = append(details, [2]string{"立体声", stereo})
	}

	return DecodeResult{
		OK:      true,
		Label:   label,
		Mod:     "WFM",
		Service: "FM 广播",
		Cols: map[string]interface{}{
			"pi": piCol, "ps": psCol, "pty": ptyCol, "af": fmt.Sprintf("%.1f", freqMHz),
		},
		Decode:  decode,
		Details: details,
	}, true
}

func decimate(src []complex128, dst []complex128, factor int) int {
	n := len(src) / factor
	if n > len(dst) {
		n = len(dst)
	}
	for i := 0; i < n; i++ {
		dst[i] = src[i*factor]
	}
	return n
}

func toneRatio(audio []float32, sr, freq float64) float64 {
	p := goertzelPower(audio, sr, freq)
	total := float64(0)
	for _, v := range audio {
		total += float64(v) * float64(v)
	}
	if total < 1e-12 {
		return 0
	}
	return p / total
}

func goertzelPower(x []float32, sr, target float64) float64 {
	n := len(x)
	if n == 0 {
		return 0
	}
	k := int(0.5 + float64(n)*target/sr)
	w := 2 * math.Pi * float64(k) / float64(n)
	cosw := math.Cos(w)
	sinw := math.Sin(w)
 coeff := 2 * cosw
	var s0, s1, s2 float64
	for _, v := range x {
		s0 = float64(v) + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	real := s1 - s2*cosw
	imag := s2 * sinw
	return real*real + imag*imag
}
