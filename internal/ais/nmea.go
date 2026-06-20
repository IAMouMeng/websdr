package ais

// Armor de-encoding for NMEA AIVDM payloads. Each character carries six bits;
// the bits concatenate MSB-first into the message bitstream. This is used by
// tests (which start from documented AIVDM sentences) and is handy for any
// future NMEA passthrough.

// payloadToBits expands an AIVDM 6-bit ASCII payload into a big-endian bit
// slice. fillBits is the count of pad bits to drop from the final character.
func payloadToBits(payload string, fillBits int) []bool {
	bits := make([]bool, 0, len(payload)*6)
	for _, c := range []byte(payload) {
		v := int(c) - 48
		if v > 40 {
			v -= 8
		}
		if v < 0 || v > 63 {
			return nil
		}
		for b := 5; b >= 0; b-- {
			bits = append(bits, v&(1<<b) != 0)
		}
	}
	if fillBits > 0 && fillBits <= len(bits) {
		bits = bits[:len(bits)-fillBits]
	}
	return bits
}
