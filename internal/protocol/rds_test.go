package protocol

import "testing"

func TestRDSSyndromeBlockA(t *testing.T) {
	data := uint16(0xABCD)
	check := rdsCheckword(data, rdsOffsetWords[0])
	raw := (uint32(data) << 10) | uint32(check)
	got, typ, ok := decodeRDSBlock(raw)
	if !ok || typ != 0 || got != data {
		t.Fatalf("block A decode failed ok=%v typ=%d got=%04X syn=%03X", ok, typ, got, rdsSyndrome(raw))
	}
}

func rdsCheckword(data uint16, offset uint16) uint16 {
	reg := uint32(data) << 10
	for i := 0; i < 16; i++ {
		if reg&0x2000000 != 0 {
			reg = (reg << 1) ^ 0x15D867A
		} else {
			reg <<= 1
		}
	}
	return (uint16(reg>>16) & 0x3FF) ^ offset
}

func TestRDSRoundTrip(t *testing.T) {
	cases := []struct {
		data uint16
		off  int
	}{
		{0xABCD, 0}, {0x0048, 1}, {0x4949, 2}, {0x464D, 3}, {0xC201, 0},
	}
	for _, c := range cases {
		check := rdsCheckword(c.data, rdsOffsetWords[c.off])
		raw := (uint32(c.data) << 10) | uint32(check)
		_, typ, ok := decodeRDSBlock(raw)
		if !ok || typ != c.off {
			t.Fatalf("data=%04X off=%d check=%03X raw=%06X syn=%03X ok=%v typ=%d", c.data, c.off, check, raw, rdsSyndrome(raw), ok, typ)
		}
	}
}

func TestParseRDSGroup0A(t *testing.T) {
	pi := uint16(0xC201)
	blockA := (uint32(pi) << 10) | uint32(rdsCheckword(pi, rdsOffsetWords[0]))
	blockBData := uint16('H')
	blockB := (uint32(blockBData) << 10) | uint32(rdsCheckword(blockBData, rdsOffsetWords[1]))
	blockCData := uint16('I')<<8 | uint16('T')
	blockC := (uint32(blockCData) << 10) | uint32(rdsCheckword(blockCData, rdsOffsetWords[2]))
	blockDData := uint16(0)
	blockD := (uint32(blockDData) << 10) | uint32(rdsCheckword(blockDData, rdsOffsetWords[3]))

	bits := make([]byte, 0, 104)
	raws := []uint32{blockA, blockB, blockC, blockD}
	for i, raw := range raws {
		_, typ, ok := decodeRDSBlock(raw)
		if !ok {
			t.Fatalf("block %d failed raw=%06X syn=%03X", i, raw, rdsSyndrome(raw))
		}
		if typ != i {
			t.Fatalf("block %d type=%d want %d", i, typ, i)
		}
	}
	for _, raw := range raws {
		for b := 25; b >= 0; b-- {
			bits = append(bits, byte((raw>>uint(b))&1))
		}
	}
	info, ok := parseRDSGroups(bits)
	if !ok {
		t.Fatal("parseRDSGroups failed")
	}
	if info.PI != "0xC201" {
		t.Fatalf("PI=%s want 0xC201", info.PI)
	}
	if info.PS != "HIT" {
		t.Fatalf("PS=%q want HIT", info.PS)
	}
}
