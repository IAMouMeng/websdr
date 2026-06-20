package ais

// fcs computes the HDLC frame check sequence used by AIS: CRC-16-CCITT in its
// reflected form (polynomial 0x8408, initial value 0xFFFF, final complement).
// Bits are processed in transmission order; the 16-bit result is sent
// least-significant-bit first after the message body.
func fcs(bits []bool) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range bits {
		in := crc&1 == 1
		if b {
			in = !in
		}
		crc >>= 1
		if in {
			crc ^= 0x8408
		}
	}
	return ^crc
}
