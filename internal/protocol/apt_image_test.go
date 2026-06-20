package protocol

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"
)

func TestDecodeAPTImageSynthetic(t *testing.T) {
	sr := APTAudioRate
	lineSamples := int(sr * 60.0 / aptLinesPerMin)
	nLines := 8
	n := lineSamples * nLines
	audio := make([]float32, n)
	for line := 0; line < nLines; line++ {
		base := line * lineSamples
		for i := 0; i < lineSamples; i++ {
			tSec := float64(i) / sr
			mod := 0.25 + 0.75*float64(i)/float64(lineSamples)
			audio[base+i] = float32(math.Sin(2*math.Pi*2400*tSec)) * float32(mod)
		}
	}
	url, lines, ok := DecodeAPTImage(audio, sr)
	if !ok || url == "" {
		t.Fatal("expected APT image")
	}
	if lines < 4 {
		t.Fatalf("lines=%d want >= 4", lines)
	}
	if !aptImageHasVisiblePixels(t, url) {
		t.Fatal("expected non-black APT preview")
	}
}

func aptImageHasVisiblePixels(t *testing.T, dataURL string) bool {
	t.Helper()
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := img.(*image.Gray)
	if !ok {
		t.Fatalf("unexpected image type %T", img)
	}
	b := g.Bounds()
	lo, hi := 255, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := int(g.GrayAt(x, y).Y)
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	return hi-lo >= 12
}
