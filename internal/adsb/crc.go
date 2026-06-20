package adsb

// Mode S parity uses a 24-bit CRC with generator polynomial 0xFFF409 (the
// implicit x^24 term making the full divisor 0x1FFF409). For DF17/DF18
// (1090ES ADS-B) the 24 parity bits are a plain CRC, so a frame is valid when
// the remainder over the whole message is zero. For the address/parity (AP)
// formats (DF0/4/5/11/16/20/21) the parity is XORed with the ICAO address, so
// the remainder of a valid frame equals that address.

const crcPoly = 0xFFF409

var crcTable [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		c := uint32(i) << 16
		for j := 0; j < 8; j++ {
			if c&0x800000 != 0 {
				c = (c << 1) ^ crcPoly
			} else {
				c <<= 1
			}
		}
		crcTable[i] = c & 0xFFFFFF
	}
}

// crc returns the 24-bit Mode S remainder over msg (7 or 14 bytes). A DF17/18
// frame is intact when this is 0; an AP-format frame returns the ICAO address.
func crc(msg []byte) uint32 {
	var rem uint32
	for _, b := range msg {
		rem = ((rem << 8) ^ crcTable[((rem>>16)^uint32(b))&0xFF]) & 0xFFFFFF
	}
	return rem
}
