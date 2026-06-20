package adsb

// Decoding of Mode S Extended Squitter (DF17/DF18) ME fields: aircraft
// identification (callsign), airborne position (with CPR-encoded lat/lon and
// barometric altitude) and airborne velocity. References: ICAO Annex 10 Vol IV
// and the dump1090 / pyModeS implementations.

// callsignChars maps the 6-bit ADS-B identification code to its character.
const callsignChars = "?ABCDEFGHIJKLMNOPQRSTUVWXYZ????? ???????????????0123456789??????"

// Message is the decoded content of one extended-squitter frame. Only the
// fields relevant to the message's type code are populated; the boolean Has*
// flags say which.
type Message struct {
	DF   int
	CA   int
	TC   int
	ICAO uint32
	Raw  []byte

	Callsign string

	HasAltitude bool
	Altitude    int // feet (barometric)

	HasPosition bool
	CPROdd      bool
	LatCPR      int
	LonCPR      int

	HasVelocity bool
	GroundSpeed float64 // knots
	Heading     float64 // degrees from true north
	VertRate    int     // feet/min, +up
}

// Decode parses a 14-byte (112-bit) DF17/DF18 frame whose CRC has already been
// verified. It returns nil for frames it does not interpret.
func Decode(msg []byte) *Message {
	if len(msg) != 14 {
		return nil
	}
	df := int(msg[0] >> 3)
	if df != 17 && df != 18 {
		return nil
	}
	m := &Message{
		DF:   df,
		CA:   int(msg[0] & 7),
		ICAO: uint32(msg[1])<<16 | uint32(msg[2])<<8 | uint32(msg[3]),
		Raw:  msg,
	}
	me := msg[4:11] // 56-bit ME field
	m.TC = int(me[0] >> 3)

	switch {
	case m.TC >= 1 && m.TC <= 4:
		m.Callsign = decodeCallsign(me)
	case m.TC >= 9 && m.TC <= 18, m.TC >= 20 && m.TC <= 22:
		m.HasAltitude, m.Altitude = decodeAltitude(me)
		m.HasPosition = true
		m.CPROdd = (me[2]>>2)&1 == 1
		m.LatCPR = int(me[2]&3)<<15 | int(me[3])<<7 | int(me[4]>>1)
		m.LonCPR = int(me[4]&1)<<16 | int(me[5])<<8 | int(me[6])
	case m.TC == 19:
		decodeVelocity(me, m)
	default:
		// Surface position (5-8) and others are not decoded here.
		if m.Callsign == "" && !m.HasPosition && !m.HasVelocity {
			return m // still useful as an ICAO presence beacon
		}
	}
	return m
}

func decodeCallsign(me []byte) string {
	// 8 characters of 6 bits each, packed into ME bits 9..56 (me[1]..me[6]).
	bits := uint64(me[1])<<40 | uint64(me[2])<<32 | uint64(me[3])<<24 |
		uint64(me[4])<<16 | uint64(me[5])<<8 | uint64(me[6])
	out := make([]byte, 8)
	for i := 0; i < 8; i++ {
		idx := (bits >> uint(42-i*6)) & 0x3F
		out[i] = callsignChars[idx]
	}
	return trimCallsign(out)
}

func trimCallsign(b []byte) string {
	// Drop trailing spaces and stray '?' placeholders.
	end := len(b)
	for end > 0 && (b[end-1] == ' ' || b[end-1] == '?') {
		end--
	}
	return string(b[:end])
}

// decodeAltitude decodes the 12-bit barometric altitude in the ME field. Only
// the common 25-ft (Q=1) encoding is handled; the Gillham (Q=0) form returns
// no altitude.
func decodeAltitude(me []byte) (bool, int) {
	ac := int(me[1])<<4 | int(me[2]>>4) // 12 bits
	if ac == 0 {
		return false, 0
	}
	q := (ac >> 4) & 1
	if q == 0 {
		return false, 0
	}
	// Remove the Q bit (bit 4) and scale: alt = 25*n - 1000 ft.
	n := (ac&0xFE0)>>1 | (ac & 0xF)
	return true, n*25 - 1000
}

func decodeVelocity(me []byte, m *Message) {
	st := int(me[0] & 7)
	if st != 1 && st != 2 { // 1/2 = ground speed; 3/4 (airspeed) not handled
		return
	}
	vEW := int(me[1]&0x03)<<8 | int(me[2]) // 10 bits
	vNS := int(me[3]&0x7F)<<3 | int(me[4]>>5)
	if vEW == 0 || vNS == 0 {
		return // velocity not available
	}
	mul := 1
	if st == 2 {
		mul = 4 // supersonic
	}
	vx := float64((vEW - 1) * mul)
	if (me[1]>>2)&1 == 1 { // east-west direction: 1 => westbound
		vx = -vx
	}
	vy := float64((vNS - 1) * mul)
	if (me[3]>>7)&1 == 1 { // north-south: 1 => southbound
		vy = -vy
	}
	m.HasVelocity = true
	m.GroundSpeed = hypot(vx, vy)
	m.Heading = headingDeg(vx, vy)

	vr := int(me[4]&7)<<6 | int(me[5]>>2) // 9 bits, ME bits 38..46
	if vr != 0 {
		rate := (vr - 1) * 64
		if (me[4]>>3)&1 == 1 { // sign: 1 => descending
			rate = -rate
		}
		m.VertRate = rate
	}
}
