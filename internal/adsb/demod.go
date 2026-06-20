package adsb

import "math"

// Mode S 1090ES is demodulated at 2 Msps: each 0.5 µs chip is one sample, each
// 1 µs data bit is two chips (Manchester). A frame is an 8 µs preamble (16
// samples) followed by 112 data bits (224 samples) — 240 samples / 120 µs in
// all. Detection is pulse-position correlation on the sample magnitude, the
// classic dump1090 approach. Only clean (CRC-zero) DF17/DF18 frames are kept;
// no single-bit error correction is attempted.

const (
	// SampleRate is the IQ rate this demodulator expects.
	SampleRate = 2000000

	preambleSamples = 16
	dataBits        = 112
	frameSamples    = preambleSamples + dataBits*2 // 240
)

// Demodulator turns a stream of unsigned-8-bit IQ blocks into Mode S frames.
// It carries a short tail between blocks so a frame straddling a block boundary
// is not lost. Not safe for concurrent use.
type Demodulator struct {
	maglut [65536]uint16
	mag    []uint16
	tail   []uint16
}

func NewDemodulator() *Demodulator {
	d := &Demodulator{}
	for i := 0; i < 256; i++ {
		fi := (float64(i) - 127.5) / 127.5
		for q := 0; q < 256; q++ {
			fq := (float64(q) - 127.5) / 127.5
			m := math.Sqrt(fi*fi+fq*fq) * 46000 // ~0..65000
			d.maglut[i<<8|q] = uint16(m)
		}
	}
	return d
}

// Reset clears the inter-block carry so the next block starts fresh.
func (d *Demodulator) Reset() { d.tail = d.tail[:0] }

// Process scans one IQ block and calls emit with each valid 14-byte frame. The
// slice handed to emit is reused after the call returns; copy it to retain.
func (d *Demodulator) Process(iq [][2]uint8, emit func(msg []byte)) {
	tailN := len(d.tail)
	total := tailN + len(iq)
	if cap(d.mag) < total {
		d.mag = make([]uint16, total)
	}
	mag := d.mag[:total]
	copy(mag, d.tail)
	for i := range iq {
		mag[tailN+i] = d.maglut[uint16(iq[i][0])<<8|uint16(iq[i][1])]
	}

	var msg [14]byte
	limit := total - frameSamples
	for j := 0; j <= limit; j++ {
		if !preambleMatch(mag[j:]) {
			continue
		}
		if d.sliceFrame(mag[j+preambleSamples:], &msg) {
			emit(msg[:])
			j += frameSamples - 1 // consumed this frame; resume after it
		}
	}

	keep := frameSamples - 1
	if total < keep {
		keep = total
	}
	d.tail = append(d.tail[:0], mag[total-keep:]...)
}

// preambleMatch tests the 8 µs pulse-position pattern: pulses at chips 0,2,7,9
// and quiet between/after. The comparisons are relative, so they self-scale to
// the signal level; a small absolute floor rejects pure noise.
func preambleMatch(m []uint16) bool {
	if !(m[0] > m[1] && m[1] < m[2] && m[2] > m[3] && m[3] < m[0] &&
		m[4] < m[0] && m[5] < m[0] && m[6] < m[0] &&
		m[7] > m[8] && m[8] < m[9] && m[9] > m[6]) {
		return false
	}
	// The four pulse peaks should clearly exceed the quiet chips 11..14.
	high := (uint32(m[0]) + uint32(m[2]) + uint32(m[7]) + uint32(m[9])) / 4
	low := (uint32(m[11]) + uint32(m[12]) + uint32(m[13]) + uint32(m[14])) / 4
	return high > low*2 && high > 600
}

// sliceFrame Manchester-decodes 112 bits starting at data[0] into msg and
// reports whether it is an intact DF17/DF18 extended squitter.
func (d *Demodulator) sliceFrame(data []uint16, msg *[14]byte) bool {
	for i := range msg {
		msg[i] = 0
	}
	for k := 0; k < dataBits; k++ {
		a := data[k*2]
		b := data[k*2+1]
		if a == b {
			return false // undecidable chip pair → reject rather than guess
		}
		if a > b {
			msg[k>>3] |= 1 << uint(7-(k&7))
		}
	}
	df := int(msg[0] >> 3)
	if df != 17 && df != 18 {
		return false
	}
	return crc(msg[:]) == 0
}
