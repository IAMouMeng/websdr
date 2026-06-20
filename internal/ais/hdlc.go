package ais

// HDLC bit-level deframing for AIS. The radio carries NRZI-encoded, bit-stuffed
// HDLC: a 0x7E flag delimits frames, a 0 is inserted after any five consecutive
// 1s, and the line uses NRZI (a 0 is a level transition, a 1 is none). This
// streaming deframer consumes raw line bits, recovers the logical bitstream,
// locates flags, removes stuffing, validates the FCS, and hands the message body
// up.
//
// Flags are found by an 8-bit pattern match, so the message content is taken
// strictly between two flags — there is no ambiguity about a flag's own border
// bits leaking into the frame.

const (
	flagPattern  = 0x7E    // 01111110
	maxFrameBits = 2048    // guard against runaway accumulation
	minFrameBits = 38 + 16 // smallest sensible message body + FCS
)

// Deframer turns a raw line-bit stream into validated message bodies.
type Deframer struct {
	havePrev bool
	prevLine bool
	reg      uint8  // last 8 logical bits, for flag detection
	bits     []bool // logical bits accumulated since the last flag
	synced   bool
	onFrame  func(body []bool)
}

func NewDeframer(onFrame func(body []bool)) *Deframer {
	return &Deframer{onFrame: onFrame}
}

func (d *Deframer) Reset() {
	d.havePrev = false
	d.reg = 0
	d.bits = d.bits[:0]
	d.synced = false
}

// PushLine consumes one raw NRZI line-state bit.
func (d *Deframer) PushLine(line bool) {
	if !d.havePrev {
		d.prevLine = line
		d.havePrev = true
		return
	}
	logical := line == d.prevLine // NRZI: no transition => logical 1
	d.prevLine = line
	d.pushLogical(logical)
}

func (d *Deframer) pushLogical(b bool) {
	d.reg <<= 1
	if b {
		d.reg |= 1
	}
	d.bits = append(d.bits, b)

	if d.reg == flagPattern {
		if d.synced && len(d.bits) >= 8 {
			d.handle(d.bits[:len(d.bits)-8]) // content lies strictly between flags
		}
		d.synced = true
		d.bits = d.bits[:0]
		return
	}
	if len(d.bits) > maxFrameBits {
		d.bits = d.bits[:0]
		d.synced = false
	}
}

func (d *Deframer) handle(content []bool) {
	frame := destuff(content)
	n := len(frame)
	if n < minFrameBits {
		return
	}
	body := frame[:n-16]
	var recv uint16
	for i := 0; i < 16; i++ {
		if frame[n-16+i] {
			recv |= 1 << uint(i) // FCS is transmitted LSB first
		}
	}
	if fcs(body) != recv {
		return
	}
	cp := make([]bool, len(body))
	copy(cp, body)
	d.onFrame(cp)
}

// destuff removes the zero inserted after every run of five consecutive 1s.
func destuff(in []bool) []bool {
	out := make([]bool, 0, len(in))
	ones := 0
	for i := 0; i < len(in); i++ {
		b := in[i]
		out = append(out, b)
		if b {
			ones++
			if ones == 5 {
				i++ // skip the stuffed 0 that follows
				ones = 0
			}
		} else {
			ones = 0
		}
	}
	return out
}
