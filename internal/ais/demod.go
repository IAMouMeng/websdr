package ais

// GMSK/FSK demodulation of one AIS channel. AIS uses GMSK at 9600 bit/s in a
// 25 kHz channel; demodulating it as FSK with a one-sample frequency
// discriminator recovers the line bits well enough for HDLC framing. Bit timing
// is a simple transition-tracking loop: the sampling instant is re-centred to
// mid-symbol whenever the discriminator changes sign, which keeps a clean burst
// aligned without a full interpolating PLL.

// SymbolRate is the AIS bit rate.
const SymbolRate = 9600.0

// ChannelDemod consumes one channel's complex baseband stream and feeds
// recovered line bits to a Deframer.
type ChannelDemod struct {
	sps     float64 // samples per symbol
	prevIQ  complex128
	haveIQ  bool
	phase   float64 // counts down to the next symbol sampling instant
	prevBit bool
	havePB  bool
	defr    *Deframer
}

// NewChannelDemod builds a demodulator for a channel sampled at sampleRate Hz.
func NewChannelDemod(sampleRate float64, onFrame func(body []bool)) *ChannelDemod {
	return &ChannelDemod{
		sps:  sampleRate / SymbolRate,
		defr: NewDeframer(onFrame),
	}
}

// Process demodulates a block of complex baseband samples.
func (c *ChannelDemod) Process(iq []complex128) {
	for _, s := range iq {
		if !c.haveIQ {
			c.prevIQ = s
			c.haveIQ = true
			continue
		}
		// Frequency discriminator: sign of the inter-sample phase change.
		d := imag(s)*real(c.prevIQ) - real(s)*imag(c.prevIQ)
		c.prevIQ = s
		bit := d > 0

		if c.havePB && bit != c.prevBit {
			c.phase = c.sps / 2 // transition: re-centre sampling at mid-symbol
		}
		c.prevBit = bit
		c.havePB = true

		c.phase--
		if c.phase <= 0 {
			c.phase += c.sps
			c.defr.PushLine(bit)
		}
	}
}

func (c *ChannelDemod) Reset() {
	c.haveIQ = false
	c.havePB = false
	c.phase = 0
	c.defr.Reset()
}
