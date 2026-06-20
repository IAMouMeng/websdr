package ais

// Application-layer decoding of an AIS binary message (ITU-R M.1371). The input
// is the message bitstream in big-endian (MSB-first) order — exactly what the
// NMEA AIVDM 6-bit armor expands to, and what the HDLC layer hands up after
// removing flags, stuffing and FCS. Only the common message types are decoded:
// 1/2/3 and 18/19 (position) and 5/24 (static/voyage).

// Report is one decoded AIS message. Numeric "not available" sentinels from the
// wire are normalised to the HasX flags / -1 here.
type Report struct {
	Type int
	MMSI uint32

	HasPos bool
	Lat    float64
	Lon    float64

	SOG     float64 // knots, -1 if n/a
	COG     float64 // degrees, -1 if n/a
	Heading int     // degrees, -1 if n/a
	NavStat int     // -1 if not applicable

	Name     string
	Callsign string
	ShipType int
}

type bitReader struct {
	bits []bool
	pos  int
}

func (r *bitReader) remaining() int { return len(r.bits) - r.pos }

func (r *bitReader) u(n int) uint64 {
	var v uint64
	for i := 0; i < n; i++ {
		v <<= 1
		if r.pos < len(r.bits) && r.bits[r.pos] {
			v |= 1
		}
		r.pos++
	}
	return v
}

func (r *bitReader) i(n int) int64 {
	v := r.u(n)
	if n < 64 && v&(1<<(n-1)) != 0 {
		return int64(v) - (1 << n)
	}
	return int64(v)
}

// sixbitChars maps the AIS 6-bit character set used for names and call signs.
const sixbitChars = "@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_ !\"#$%&'()*+,-./0123456789:;<=>?"

func (r *bitReader) text(chars int) string {
	out := make([]byte, 0, chars)
	for i := 0; i < chars; i++ {
		out = append(out, sixbitChars[r.u(6)])
	}
	return trimText(out)
}

func trimText(b []byte) string {
	end := len(b)
	for end > 0 && (b[end-1] == ' ' || b[end-1] == '@') {
		end--
	}
	// '@' is the AIS pad/null; also strip any embedded trailing nulls.
	return string(b[:end])
}

// DecodePayload decodes a big-endian message bitstream. It returns nil when the
// message is too short or of a type this package does not handle.
func DecodePayload(bits []bool) *Report {
	if len(bits) < 38 { // need at least type(6)+repeat(2)+mmsi(30)
		return nil
	}
	r := &bitReader{bits: bits}
	rep := &Report{
		SOG:     -1,
		COG:     -1,
		Heading: -1,
		NavStat: -1,
	}
	rep.Type = int(r.u(6))
	r.u(2) // repeat indicator
	rep.MMSI = uint32(r.u(30))

	switch rep.Type {
	case 1, 2, 3:
		decodePositionA(r, rep)
	case 18:
		decodePositionB(r, rep)
	case 19:
		decodePositionB(r, rep)
		if r.remaining() >= 4+120 {
			r.u(4) // regional/spare before name in type 19
			rep.Name = r.text(20)
		}
	case 5:
		decodeStatic5(r, rep)
	case 24:
		decodeStatic24(r, rep)
	default:
		return rep // unknown type, but MMSI is still useful
	}
	return rep
}

func decodePositionA(r *bitReader, rep *Report) {
	if r.remaining() < 4+8+10+1+28+27+12+9 {
		return
	}
	rep.NavStat = int(r.u(4))
	r.i(8)  // rate of turn
	sog := r.u(10)
	r.u(1) // position accuracy
	lon := r.i(28)
	lat := r.i(27)
	cog := r.u(12)
	hdg := r.u(9)
	fillKinematics(rep, sog, lon, lat, cog, hdg)
}

func decodePositionB(r *bitReader, rep *Report) {
	if r.remaining() < 8+10+1+28+27+12+9 {
		return
	}
	r.u(8) // reserved
	sog := r.u(10)
	r.u(1) // accuracy
	lon := r.i(28)
	lat := r.i(27)
	cog := r.u(12)
	hdg := r.u(9)
	fillKinematics(rep, sog, lon, lat, cog, hdg)
}

func fillKinematics(rep *Report, sog uint64, lon, lat int64, cog, hdg uint64) {
	if sog != 1023 {
		rep.SOG = float64(sog) / 10.0
	}
	if lon != 181*600000 && lat != 91*600000 {
		rep.Lon = float64(lon) / 600000.0
		rep.Lat = float64(lat) / 600000.0
		rep.HasPos = rep.Lat >= -90 && rep.Lat <= 90 && rep.Lon >= -180 && rep.Lon <= 180
	}
	if cog != 3600 {
		rep.COG = float64(cog) / 10.0
	}
	if hdg != 511 {
		rep.Heading = int(hdg)
	}
}

func decodeStatic5(r *bitReader, rep *Report) {
	if r.remaining() < 2+30+42+120+8 {
		return
	}
	r.u(2)  // AIS version
	r.u(30) // IMO number
	rep.Callsign = r.text(7)
	rep.Name = r.text(20)
	rep.ShipType = int(r.u(8))
}

func decodeStatic24(r *bitReader, rep *Report) {
	if r.remaining() < 2 {
		return
	}
	part := r.u(2)
	if part == 0 { // Part A: name
		if r.remaining() >= 120 {
			rep.Name = r.text(20)
		}
		return
	}
	// Part B: type + call sign
	if r.remaining() >= 8+18+42 {
		rep.ShipType = int(r.u(8))
		r.u(18) // vendor ID
		rep.Callsign = r.text(7)
	}
}
