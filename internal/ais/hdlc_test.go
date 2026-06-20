package ais

import "testing"

// hdlcEncode is the inverse of the Deframer: it wraps a message body in a
// flag-delimited, bit-stuffed, NRZI-encoded line-bit stream. Used only by tests
// to prove the deframing pipeline round-trips.
func hdlcEncode(body []bool) []bool {
	flag := []bool{false, true, true, true, true, true, true, false}

	payload := append([]bool{}, body...)
	f := fcs(body)
	for i := 0; i < 16; i++ {
		payload = append(payload, f&(1<<uint(i)) != 0)
	}

	logical := append([]bool{}, flag...)
	ones := 0
	for _, b := range payload {
		logical = append(logical, b)
		if b {
			ones++
			if ones == 5 {
				logical = append(logical, false) // insert stuffing zero
				ones = 0
			}
		} else {
			ones = 0
		}
	}
	logical = append(logical, flag...)

	// NRZI encode: logical 0 => transition, logical 1 => hold.
	line := make([]bool, 0, len(logical)+1)
	cur := false
	line = append(line, cur)
	for _, b := range logical {
		if !b {
			cur = !cur
		}
		line = append(line, cur)
	}
	return line
}

func TestHDLCRoundTrip(t *testing.T) {
	body := payloadToBits("177KQJ5000G?tO`K>RA1wUbN0TKH", 0)
	if len(body) == 0 {
		t.Fatal("bad test payload")
	}
	line := hdlcEncode(body)

	var got [][]bool
	d := NewDeframer(func(data []bool) {
		cp := append([]bool{}, data...)
		got = append(got, cp)
	})
	// Feed idle (logical ones = no transitions) before and after the frame.
	for i := 0; i < 8; i++ {
		d.PushLine(false)
	}
	for _, b := range line {
		d.PushLine(b)
	}
	for i := 0; i < 8; i++ {
		d.PushLine(line[len(line)-1])
	}

	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1", len(got))
	}
	if len(got[0]) != len(body) {
		t.Fatalf("body length = %d, want %d", len(got[0]), len(body))
	}
	for i := range body {
		if got[0][i] != body[i] {
			t.Fatalf("body bit %d differs", i)
		}
	}
	if r := DecodePayload(got[0]); r == nil || r.MMSI != 477553000 {
		t.Fatalf("decoded MMSI wrong: %+v", r)
	}
}

func TestHDLCRejectsCorrupt(t *testing.T) {
	body := payloadToBits("177KQJ5000G?tO`K>RA1wUbN0TKH", 0)
	line := hdlcEncode(body)
	line[20] = !line[20] // flip a bit inside the frame

	got := 0
	d := NewDeframer(func(data []bool) { got++ })
	for _, b := range line {
		d.PushLine(b)
	}
	if got != 0 {
		t.Errorf("corrupt frame accepted (%d frames)", got)
	}
}
