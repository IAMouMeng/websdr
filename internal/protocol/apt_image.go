package protocol

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/iamoumeng/websdr/internal/dsp"
)

const (
	APTAudioRate            = 19200.0
	aptLinesPerMin          = 120.0
	aptPixelsPerLine        = 909
	aptPreviewPixelsPerLine = 360 // websocket preview width (per channel)
	aptVisibleHz            = 2400.0
	aptIRHz                 = 2080.0
)

// APTLineCount estimates decoded line count from FM audio length.
func APTLineCount(fmAudio []float32, sampleRate float64) int {
	if sampleRate <= 0 {
		sampleRate = APTAudioRate
	}
	lineSamples := int(sampleRate * 60.0 / aptLinesPerMin)
	if lineSamples <= 0 {
		return 0
	}
	return len(fmAudio) / lineSamples
}

// DecodeAPTImage builds a visible+IR grayscale APT strip from FM-demodulated audio.
func DecodeAPTImage(fmAudio []float32, sampleRate float64) (dataURL string, lines int, ok bool) {
	return decodeAPTImage(fmAudio, sampleRate, aptPreviewPixelsPerLine)
}

func decodeAPTImage(fmAudio []float32, sampleRate float64, pixelsPerLine int) (dataURL string, lines int, ok bool) {
	if sampleRate <= 0 {
		sampleRate = APTAudioRate
	}
	if len(fmAudio) < int(sampleRate*0.6) {
		return "", 0, false
	}
	fmAudio = normalizeFMAudio(fmAudio)

	visEnv := envelopeAM(fmAudio, sampleRate, aptVisibleHz)
	irEnv := envelopeAM(fmAudio, sampleRate, aptIRHz)

	phase := findBestLinePhase(visEnv, sampleRate)
	visLines := extractLinesAt(visEnv, sampleRate, phase)
	irLines := extractLinesAt(irEnv, sampleRate, phase)
	if len(visLines) == 0 {
		return "", 0, false
	}
	n := len(visLines)
	if len(irLines) < n {
		n = len(irLines)
	}
	if n < 2 {
		return "", 0, false
	}
	visLines = visLines[:n]
	irLines = irLines[:n]

	lo, hi := globalEnvelopeRange(visLines, irLines)
	if hi-lo < 1e-8 {
		lo, hi = perLineEnvelopeRange(visLines, irLines)
	}
	if hi-lo < 1e-8 {
		return "", n, false
	}

	w := pixelsPerLine * 2
	img := image.NewGray(image.Rect(0, 0, w, n))
	for y := 0; y < n; y++ {
		drawLine(img, 0, y, resampleLineGlobal(visLines[y], pixelsPerLine, lo, hi))
		drawLine(img, pixelsPerLine, y, resampleLineGlobal(irLines[y], pixelsPerLine, lo, hi))
	}
	stretchImage(img)

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return "", n, false
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), n, true
}

func envelopeAM(audio []float32, sr, freq float64) []float32 {
	n := len(audio)
	env := make([]float32, n)
	const alpha = 0.02
	var lpI, lpQ float32
	var phase float64
	for i := 0; i < n; i++ {
		phase += 2 * math.Pi * freq / sr
		iVal := audio[i] * float32(math.Cos(phase))
		qVal := audio[i] * float32(math.Sin(phase))
		lpI = lpI*(1-alpha) + iVal*alpha
		lpQ = lpQ*(1-alpha) + qVal*alpha
		env[i] = float32(math.Hypot(float64(lpI), float64(lpQ)))
	}
	return env
}

func findBestLinePhase(env []float32, sr float64) int {
	lineSamples := int(sr * 60.0 / aptLinesPerMin)
	if lineSamples < aptPixelsPerLine*2 || len(env) < lineSamples*3 {
		return 0
	}
	step := lineSamples / 48
	if step < 1 {
		step = 1
	}
	bestPhase, bestScore := 0, -1.0
	for phase := 0; phase < lineSamples; phase += step {
		lines := extractLinesAt(env, sr, phase)
		if len(lines) < 3 {
			continue
		}
		score := linePhaseScore(lines)
		if score > bestScore {
			bestScore = score
			bestPhase = phase
		}
	}
	return bestPhase
}

func linePhaseScore(lines [][]float32) float64 {
	if len(lines) < 2 {
		return 0
	}
	var corrSum float64
	pairs := 0
	for i := 1; i < len(lines); i++ {
		a, b := lines[i-1], lines[i]
		n := len(a)
		if len(b) < n {
			n = len(b)
		}
		if n < 16 {
			continue
		}
		var sumA, sumB float64
		for j := 0; j < n; j++ {
			sumA += float64(a[j])
			sumB += float64(b[j])
		}
		meanA, meanB := sumA/float64(n), sumB/float64(n)
		var va, vb, cov float64
		for j := 0; j < n; j++ {
			da := float64(a[j]) - meanA
			db := float64(b[j]) - meanB
			va += da * da
			vb += db * db
			cov += da * db
		}
		denom := math.Sqrt(va * vb)
		if denom < 1e-12 {
			continue
		}
		corrSum += cov / denom
		pairs++
	}
	if pairs == 0 {
		return 0
	}
	return corrSum / float64(pairs)
}

func extractLinesAt(env []float32, sr float64, phase int) [][]float32 {
	lineSamples := int(sr * 60.0 / aptLinesPerMin)
	if lineSamples < aptPixelsPerLine*2 {
		return nil
	}
	skip := lineSamples / 16
	use := lineSamples - skip*2
	if use < aptPixelsPerLine {
		return nil
	}

	var out [][]float32
	for off := phase; off+lineSamples <= len(env); off += lineSamples {
		seg := env[off+skip : off+skip+use]
		dup := make([]float32, len(seg))
		copy(dup, seg)
		out = append(out, dup)
	}
	return out
}

func globalEnvelopeRange(sets ...[][]float32) (lo, hi float32) {
	first := true
	for _, lines := range sets {
		for _, seg := range lines {
			for _, v := range seg {
				if first {
					lo, hi = v, v
					first = false
					continue
				}
				if v < lo {
					lo = v
				}
				if v > hi {
					hi = v
				}
			}
		}
	}
	return lo, hi
}

func resampleLineGlobal(seg []float32, pixels int, lo, hi float32) []uint8 {
	out := make([]uint8, pixels)
	if len(seg) == 0 {
		return out
	}
	span := hi - lo
	for i := 0; i < pixels; i++ {
		pos := float64(i) * float64(len(seg)-1) / float64(pixels-1)
		j := int(pos)
		frac := pos - float64(j)
		var v float32
		if j >= len(seg)-1 {
			v = seg[len(seg)-1]
		} else {
			v = seg[j]*(1-float32(frac)) + seg[j+1]*float32(frac)
		}
		if span < 1e-8 {
			out[i] = 128
			continue
		}
		n := (v - lo) / span
		if n < 0 {
			n = 0
		}
		if n > 1 {
			n = 1
		}
		out[i] = uint8(n*255 + 0.5)
	}
	return out
}

func normalizeFMAudio(audio []float32) []float32 {
	if len(audio) == 0 {
		return audio
	}
	var sum float64
	for _, v := range audio {
		sum += float64(v)
	}
	mean := float32(sum / float64(len(audio)))
	out := make([]float32, len(audio))
	var peak float32
	for i, v := range audio {
		v -= mean
		out[i] = v
		av := v
		if av < 0 {
			av = -av
		}
		if av > peak {
			peak = av
		}
	}
	if peak < 1e-8 {
		return out
	}
	inv := 1 / peak
	for i := range out {
		out[i] *= inv
	}
	return out
}

func perLineEnvelopeRange(sets ...[][]float32) (lo, hi float32) {
	first := true
	for _, lines := range sets {
		for _, seg := range lines {
			if len(seg) == 0 {
				continue
			}
			lineLo, lineHi := seg[0], seg[0]
			for _, v := range seg {
				if v < lineLo {
					lineLo = v
				}
				if v > lineHi {
					lineHi = v
				}
			}
			if first {
				lo, hi = lineLo, lineHi
				first = false
				continue
			}
			if lineLo < lo {
				lo = lineLo
			}
			if lineHi > hi {
				hi = lineHi
			}
		}
	}
	return lo, hi
}

func stretchImage(img *image.Gray) {
	b := img.Bounds()
	lo, hi := byte(255), byte(0)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := img.GrayAt(x, y).Y
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if hi <= lo {
		return
	}
	margin := float64(hi-lo) * 0.02
	if hi-lo <= 20 {
		margin = 0
	}
	loF := float64(lo) + margin
	hiF := float64(hi) - margin
	if hiF <= loF {
		loF, hiF = float64(lo), float64(hi)
	}
	span := hiF - loF
	if span < 1 {
		span = 1
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := float64(img.GrayAt(x, y).Y)
			if v < loF {
				v = loF
			}
			if v > hiF {
				v = hiF
			}
			n := uint8((v-loF)/span*255 + 0.5)
			img.SetGray(x, y, color.Gray{Y: n})
		}
	}
}

func imageHasContrast(img *image.Gray) bool {
	b := img.Bounds()
	if b.Dy() == 0 || b.Dx() == 0 {
		return false
	}
	lo, hi := byte(255), byte(0)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := img.GrayAt(x, y).Y
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	return int(hi)-int(lo) >= 16
}

func drawLine(img *image.Gray, x0, y int, line []uint8) {
	for x, v := range line {
		img.SetGray(x0+x, y, color.Gray{Y: v})
	}
}

func mildStretch(img *image.Gray) {
	stretchImage(img)
}

// FMAudioFromIQ mixes, decimates and FM-demodulates IQ for APT work.
func FMAudioFromIQ(iq []complex128, sr, offsetHz float64) []float32 {
	if len(iq) == 0 {
		return nil
	}
	work := make([]complex128, len(iq))
	copy(work, iq)
	var phase float64
	dsp.MixDown(work, offsetHz, sr, &phase)

	factor := int(sr / APTAudioRate)
	if factor < 1 {
		factor = 1
	}
	dec := make([]complex128, len(work)/factor+1)
	n := decimate(work, dec, factor)
	if n < 256 {
		return nil
	}
	workSR := sr / float64(factor)

	audio := make([]float32, n)
	var fmPrev complex128
	dsp.FMDemod(dec[:n], 17_000, workSR, &fmPrev, audio)
	return audio
}

func AppendFMAudio(dst []float32, chunk []float32, maxLen int) []float32 {
	dst = append(dst, chunk...)
	if len(dst) > maxLen {
		dst = dst[len(dst)-maxLen:]
	}
	return dst
}

func HasType(b Band, typ string) bool {
	for _, t := range b.Types {
		if t == typ {
			return true
		}
	}
	return false
}
