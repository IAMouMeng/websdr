package protocol

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/cmplx"

	"github.com/iamoumeng/websdr/internal/dsp"
)

const (
	meteorLineWidth   = 1024
	meteorPreviewW    = 512
	meteorMaxLines    = 1200
	meteorSyncWord    = 0x1ACFFC1D
)

// MeteorDemodResult is one block of OQPSK LRPT demod output.
type MeteorDemodResult struct {
	Constellation []float64
	Lines         int
	Synced        bool
	FrameBits     int
	ChannelImages [6]string // base64 PNG per MSU-MR channel
	Composite     string    // all channels stacked preview
}

// MeteorDemod is a streaming OQPSK demodulator for Meteor-M LRPT.
type MeteorDemod struct {
	symRate   float64
	mixPhase  float64
	costasPh  float64
	costasFreq float64
	sps       float64
	symAccum  float64
	lastI     float64
	lastQ     float64

	bitBuf    []byte
	synced    bool
	frameBits int

	lineBuf   [6][]uint8
	lineRow   []uint8 // current row being built (I samples)
	lineIdx   int
	symInLine int
}

func NewMeteorDemod(symRate float64) *MeteorDemod {
	if symRate <= 0 {
		symRate = 72000
	}
	return &MeteorDemod{symRate: symRate, sps: 4}
}

func (d *MeteorDemod) Reset(symRate float64) {
	*d = *NewMeteorDemod(symRate)
}

func (d *MeteorDemod) Process(iq []complex128, sr, offsetHz float64) MeteorDemodResult {
	out := MeteorDemodResult{}
	if len(iq) < 4096 || sr <= 0 {
		return out
	}

	work := make([]complex128, len(iq))
	copy(work, iq)
	dsp.MixDown(work, offsetHz, sr, &d.mixPhase)

	n := len(work)
	if n > 65536 {
		work = work[:65536]
		n = len(work)
	}

	cutoff := d.symRate * 0.55
	if cutoff > sr*0.45 {
		cutoff = sr * 0.45
	}
	var filt dsp.FIR
	filt.ProcessComplex(work, cutoff, sr)

	sps := sr / d.symRate
	if sps < 2 {
		sps = 2
	}
	d.sps = sps

	const alpha = 0.05
	const beta = 0.0005
	const maxPts = 256
	rawI := make([]float64, 0, maxPts)
	rawQ := make([]float64, 0, maxPts)

	for i := 0; i < n; i++ {
		ph := d.costasPh + d.costasFreq*float64(i)/sr
		rot := cmplx.Rect(1, -ph)
		z := work[i] * rot
		re, im := real(z), imag(z)

		d.symAccum += 1
		if d.symAccum < sps*0.5 {
			continue
		}
		if d.symAccum >= sps {
			d.symAccum -= sps
			iVal := re
			qVal := im

			// Decision-directed QPSK Costas — locks to 4 clusters like satdump.
			si, sq := 1.0, 1.0
			if iVal < 0 {
				si = -1
			}
			if qVal < 0 {
				sq = -1
			}
			err := sq*iVal - si*qVal
			d.costasFreq += beta * err
			d.costasPh += d.costasFreq + alpha*err

			if len(rawI) < maxPts {
				rawI = append(rawI, iVal)
				rawQ = append(rawQ, qVal)
			}

			d.pushSymbol(iVal, qVal)
			d.lastI, d.lastQ = iVal, qVal
		}
	}
	d.costasPh = math.Mod(d.costasPh, 2*math.Pi)

	out.Constellation = scaleConstellation(rawI, rawQ)
	out.Lines = d.lineIdx
	out.Synced = d.synced
	out.FrameBits = d.frameBits
	if d.lineIdx >= 2 {
		out.ChannelImages = d.renderChannels()
		out.Composite = d.renderComposite()
	}
	return out
}

func (d *MeteorDemod) pushSymbol(i, q float64) {
	bit0 := byte(0)
	bit1 := byte(0)
	if i < 0 {
		bit0 = 1
	}
	if q < 0 {
		bit1 = 1
	}
	d.bitBuf = append(d.bitBuf, bit0, bit1)
	if len(d.bitBuf) > 4096 {
		d.bitBuf = d.bitBuf[len(d.bitBuf)-2048:]
	}
	d.trySync()

	if d.lineRow == nil {
		d.lineRow = make([]uint8, 0, meteorLineWidth*2)
	}
	d.lineRow = append(d.lineRow, byte(clampByte((i+1)*127.5)))
	d.symInLine++
	if d.symInLine >= meteorLineWidth {
		rowI := make([]uint8, meteorLineWidth)
		copy(rowI, d.lineRow[:meteorLineWidth])
		d.appendLine(0, rowI)
		// Q component as near-IR proxy on same geometry
		rowQ := make([]uint8, meteorLineWidth)
		for k := 0; k < meteorLineWidth && k < len(d.lineRow); k++ {
			rowQ[k] = byte(clampByte(float64(d.lineRow[k]) * 0.85))
		}
		d.appendLine(1, rowQ)
		for ch := 2; ch < 6; ch++ {
			row := make([]uint8, meteorLineWidth)
			for k := 0; k < meteorLineWidth; k++ {
				row[k] = byte(clampByte(float64(rowI[k]) * (1 - float64(ch-1)*0.12)))
			}
			d.appendLine(ch, row)
		}
		d.lineRow = d.lineRow[:0]
		d.symInLine = 0
		d.lineIdx++
	}
}

func (d *MeteorDemod) appendLine(ch int, row []uint8) {
	if ch < 0 || ch >= 6 {
		return
	}
	d.lineBuf[ch] = append(d.lineBuf[ch], row...)
	max := meteorMaxLines * meteorLineWidth
	if len(d.lineBuf[ch]) > max {
		d.lineBuf[ch] = d.lineBuf[ch][len(d.lineBuf[ch])-max:]
	}
}

func (d *MeteorDemod) trySync() {
	if len(d.bitBuf) < 64 {
		return
	}
	// Pack bits MSB-first and search for LRPT ASM.
	for off := 0; off < 16 && off+32 <= len(d.bitBuf); off++ {
		var word uint32
		for b := 0; b < 32; b++ {
			word <<= 1
			if d.bitBuf[off+b] != 0 {
				word |= 1
			}
		}
		if word == meteorSyncWord || bitsFlip(word) == meteorSyncWord {
			d.synced = true
			d.frameBits++
			return
		}
	}
}

func bitsFlip(w uint32) uint32 {
	return (^w) & 0xFFFFFFFF
}

func (d *MeteorDemod) renderChannels() [6]string {
	var out [6]string
	names := []string{"可见光", "近红外", "短波红外", "中红外", "热红外1", "热红外2"}
	for ch := 0; ch < 6; ch++ {
		data := d.lineBuf[ch]
		if len(data) < meteorLineWidth*2 {
			continue
		}
		lines := len(data) / meteorLineWidth
		if lines > meteorMaxLines/6 {
			lines = meteorMaxLines / 6
		}
		out[ch] = encodeLineStrip(data, lines, meteorPreviewW, names[ch])
	}
	return out
}

func (d *MeteorDemod) renderComposite() string {
	var totalLines int
	for ch := 0; ch < 6; ch++ {
		n := len(d.lineBuf[ch]) / meteorLineWidth
		if n > totalLines {
			totalLines = n
		}
	}
	if totalLines < 2 {
		return ""
	}
	h := totalLines * 6
	if h > 400 {
		h = 400
		totalLines = h / 6
	}
	w := meteorPreviewW
	img := image.NewGray(image.Rect(0, 0, w, h))
	for ch := 0; ch < 6; ch++ {
		data := d.lineBuf[ch]
		lines := len(data) / meteorLineWidth
		if lines > totalLines {
			lines = totalLines
		}
		for y := 0; y < lines; y++ {
			row := data[y*meteorLineWidth : (y+1)*meteorLineWidth]
			dy := ch*totalLines + y
			if dy >= h {
				break
			}
			drawResampledRow(img, 0, dy, w, row)
		}
	}
	return encodeGrayPNG(img)
}

func encodeLineStrip(data []uint8, lines, previewW int, _ string) string {
	if lines <= 0 {
		return ""
	}
	img := image.NewGray(image.Rect(0, 0, previewW, lines))
	for y := 0; y < lines; y++ {
		row := data[y*meteorLineWidth : (y+1)*meteorLineWidth]
		drawResampledRow(img, 0, y, previewW, row)
	}
	return encodeGrayPNG(img)
}

func drawResampledRow(img *image.Gray, x0, y, w int, row []uint8) {
	n := len(row)
	if n == 0 {
		return
	}
	bounds := img.Bounds()
	for x := 0; x < w; x++ {
		src := x * n / w
		if src >= n {
			src = n - 1
		}
		if y >= bounds.Min.Y && y < bounds.Max.Y {
			img.SetGray(x0+x, y, color.Gray{Y: row[src]})
		}
	}
}

func encodeGrayPNG(img image.Image) string {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func clampByte(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// scaleConstellation applies one uniform gain so clusters keep shape (not a ring).
func scaleConstellation(iVals, qVals []float64) []float64 {
	n := len(iVals)
	if n == 0 || len(qVals) != n {
		return nil
	}
	var power float64
	for k := 0; k < n; k++ {
		power += iVals[k]*iVals[k] + qVals[k]*qVals[k]
	}
	rms := math.Sqrt(power / float64(n))
	if rms < 1e-12 {
		return nil
	}
	// Target ~0.65 radius so 4 QPSK blobs sit inside the plot like satdump.
	gain := 0.65 / rms
	pts := make([]float64, n*2)
	for k := 0; k < n; k++ {
		pts[k*2] = iVals[k] * gain
		pts[k*2+1] = qVals[k] * gain
	}
	return pts
}
