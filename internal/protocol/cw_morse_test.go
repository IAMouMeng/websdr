package protocol

import "testing"

func synthWaterfallMorse(rows int, wpm float64, letter string) ([]Peak, [][]float32, Band) {
	const (
		centerHz = uint64(28_300_000)
		rateHz   = uint(2_048_000)
		nBins    = 1024
		peakHz   = uint64(28_150_000)
	)

	code := morseForLetter(rune(letter[0]))

	ditRows := int(60.0 / (50.0 * wpm) * 3.0 / float64(rows) * float64(rows))
	if ditRows < 2 {
		ditRows = 2
	}
	dahRows := ditRows * 3
	gapRows := ditRows

	spectra := make([][]float32, 0, rows)
	floor := float32(-95)
	on := float32(-55)

	for _, ch := range code {
		switch ch {
		case '.':
			for i := 0; i < ditRows && len(spectra) < rows; i++ {
				spectra = append(spectra, synthRow(nBins, peakHz, centerHz, rateHz, on, floor))
			}
			for i := 0; i < gapRows && len(spectra) < rows; i++ {
				spectra = append(spectra, synthRow(nBins, peakHz, centerHz, rateHz, floor, floor))
			}
		case '-':
			for i := 0; i < dahRows && len(spectra) < rows; i++ {
				spectra = append(spectra, synthRow(nBins, peakHz, centerHz, rateHz, on, floor))
			}
			for i := 0; i < gapRows && len(spectra) < rows; i++ {
				spectra = append(spectra, synthRow(nBins, peakHz, centerHz, rateHz, floor, floor))
			}
		}
	}
	for len(spectra) < rows {
		spectra = append(spectra, synthRow(nBins, peakHz, centerHz, rateHz, floor, floor))
	}

	peak := Peak{FreqHz: peakHz, PowerDB: -55, BwHz: 400}
	band := Band{Name: "10m CW", CenterHz: centerHz, RateHz: rateHz, Types: []string{"cw"}}
	return []Peak{peak}, spectra, band
}

func synthRow(nBins int, peakHz, centerHz uint64, rateHz uint, on, floor float32) []float32 {
	row := make([]float32, nBins)
	for i := range row {
		row[i] = floor
	}
	bin := freqToBin(peakHz, centerHz, rateHz, nBins)
	for d := -1; d <= 1; d++ {
		if b := bin + d; b >= 0 && b < nBins {
			row[b] = on
		}
	}
	return row
}

func morseForLetter(ch rune) string {
	for sym, lit := range morseTable {
		if lit == string(ch) {
			return sym
		}
	}
	return ""
}

func TestDecodeMorseFromWaterfall(t *testing.T) {
	_, spectra, band := synthWaterfallMorse(90, 18, "S")
	peak := Peak{FreqHz: 28_150_000, PowerDB: -55, BwHz: 400}
	dr, ok := TryCWFromWaterfall(peak, band, spectra, band.CenterHz, band.RateHz)
	if !ok {
		t.Fatal("expected morse decode from synthetic waterfall")
	}
	if dr.Decode == nil || dr.Decode.Metric != "S" {
		t.Fatalf("got metric=%q", dr.Decode.Metric)
	}
}

func TestTryCWPeakUsesWaterfall(t *testing.T) {
	_, spectra, band := synthWaterfallMorse(90, 18, "O")
	peak := Peak{FreqHz: 28_150_000, PowerDB: -55, BwHz: 400}
	dr, ok := TryCWPeak(peak, band, nil, float64(band.RateHz), band.CenterHz, spectra)
	if !ok || dr.Decode == nil || dr.Decode.Metric != "O" {
		t.Fatalf("waterfall morse path failed: ok=%v metric=%v", ok, dr.Decode)
	}
}
