package protocol

import (
	"math"
	"sort"
)

// NOAA / Metop APT downlink center frequencies (Hz).
// Most NOAA APT transmitters are off-air; keep channels for manual APT listen only.
var aptDownlinks = []uint64{
	137_100_000, // NOAA 19
	137_500_000, // NOAA 17 (legacy)
	137_620_000, // NOAA 15 (APT off)
	137_912_500, // NOAA 18
}

const (
	aptSnapMaxHz   = uint64(25_000) // only snap peaks already on/near a downlink
	aptMergeMaxHz  = float64(35_000) // merge spurs from one wide FM carrier
	aptMinBwHz     = float64(25_000)
	aptMaxBwHz     = float64(90_000)
	aptMinPowerDB  = -78
)

// IsNearAPTDownlink reports whether freq is within aptSnapMaxHz of a known channel.
func IsNearAPTDownlink(freqHz uint64) bool {
	return distNearest(freqHz, aptDownlinks) <= aptSnapMaxHz
}

// SnapAPTDownlink maps a detected frequency onto the nearest known APT channel
// when it is close enough (same wide-FM pass, spurious FFT peak).
func SnapAPTDownlink(freqHz uint64) uint64 {
	if freqHz < 136_000_000 || freqHz > 138_500_000 {
		return freqHz
	}
	best := freqHz
	bestDist := uint64(aptSnapMaxHz + 1)
	for _, ch := range aptDownlinks {
		var dist uint64
		if freqHz >= ch {
			dist = freqHz - ch
		} else {
			dist = ch - freqHz
		}
		if dist < bestDist {
			bestDist = dist
			best = ch
		}
	}
	if bestDist <= aptSnapMaxHz {
		return best
	}
	return freqHz
}

// CollapseAPTPeaks merges multiple FFT peaks from one wide APT carrier and snaps
// survivors onto standard downlink frequencies.
func CollapseAPTPeaks(peaks []Peak) []Peak {
	var apt []Peak
	for _, p := range peaks {
		if p.FreqHz >= 136_000_000 && p.FreqHz <= 138_500_000 {
			apt = append(apt, p)
		}
	}
	if len(apt) == 0 {
		return peaks
	}

	sort.Slice(apt, func(i, j int) bool { return apt[i].PowerDB > apt[j].PowerDB })

	var kept []Peak
	var passthrough []Peak
	for _, p := range apt {
		if p.BwHz > 0 && (p.BwHz < aptMinBwHz || p.BwHz > aptMaxBwHz) {
			passthrough = append(passthrough, p)
			continue
		}
		if !IsNearAPTDownlink(p.FreqHz) {
			passthrough = append(passthrough, p)
			continue
		}
		dup := false
		snapped := SnapAPTDownlink(p.FreqHz)
		for _, k := range kept {
			if math.Abs(float64(snapped)-float64(k.FreqHz)) < aptMergeMaxHz {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		p.FreqHz = snapped
		kept = append(kept, p)
	}

	// Preserve non-APT peaks (e.g. amateur satellite band handled elsewhere).
	var other []Peak
	for _, p := range peaks {
		if p.FreqHz < 136_000_000 || p.FreqHz > 138_500_000 {
			other = append(other, p)
		}
	}
	return append(append(other, passthrough...), kept...)
}

func IsWeatherAPTBand(band Band) bool {
	return band.CenterHz >= 136_500_000 && band.CenterHz <= 137_500_000 && HasType(band, "satellite")
}
