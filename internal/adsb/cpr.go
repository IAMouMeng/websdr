package adsb

import "math"

// Compact Position Reporting. ADS-B sends latitude/longitude as 17-bit fractions
// within a grid whose cell changes between "even" and "odd" frames. A globally
// unambiguous position needs one even and one odd frame close in time; once a
// position is known, a single frame can be locally decoded against it.

const nz = 15 // number of geographic latitude zones between equator and a pole

// cprNL returns the number of longitude zones at the given latitude.
func cprNL(lat float64) float64 {
	lat = math.Abs(lat)
	if lat < 1e-9 {
		return 59
	}
	if lat >= 87.0 {
		if lat > 87.0 {
			return 1
		}
		return 2
	}
	a := 1 - math.Cos(math.Pi/(2*nz))
	b := math.Cos(math.Pi / 180.0 * lat)
	return math.Floor(2 * math.Pi / math.Acos(1-a/(b*b)))
}

func cprMod(a, b float64) float64 {
	r := math.Mod(a, b)
	if r < 0 {
		r += b
	}
	return r
}

// cprGlobal recovers an absolute position from a paired even and odd frame.
// evenNewer says which of the two arrived most recently (it is used as the
// reference frame). ok is false when the pair is inconsistent.
func cprGlobal(latEvenCPR, lonEvenCPR, latOddCPR, lonOddCPR int, evenNewer bool) (lat, lon float64, ok bool) {
	const n = 1 << 17
	latE := float64(latEvenCPR) / n
	lonE := float64(lonEvenCPR) / n
	latO := float64(latOddCPR) / n
	lonO := float64(lonOddCPR) / n

	const dLatE = 360.0 / (4 * nz)
	const dLatO = 360.0 / (4*nz - 1)

	j := math.Floor(59*latE - 60*latO + 0.5)
	rlatE := dLatE * (cprMod(j, 60) + latE)
	rlatO := dLatO * (cprMod(j, 59) + latO)
	if rlatE >= 270 {
		rlatE -= 360
	}
	if rlatO >= 270 {
		rlatO -= 360
	}
	if cprNL(rlatE) != cprNL(rlatO) {
		return 0, 0, false // straddles a latitude zone boundary; wait for a fresh pair
	}

	if evenNewer {
		lat = rlatE
		nl := cprNL(rlatE)
		ni := math.Max(nl, 1)
		m := math.Floor(lonE*(nl-1) - lonO*nl + 0.5)
		lon = (360.0 / ni) * (cprMod(m, ni) + lonE)
	} else {
		lat = rlatO
		nl := cprNL(rlatO)
		ni := math.Max(nl-1, 1)
		m := math.Floor(lonE*(nl-1) - lonO*nl + 0.5)
		lon = (360.0 / ni) * (cprMod(m, ni) + lonO)
	}
	if lon >= 180 {
		lon -= 360
	}
	return lat, lon, true
}

// cprLocal recovers a position from a single frame using a nearby reference
// (the last known position of the same aircraft, or the receiver site).
func cprLocal(latCPR, lonCPR int, odd bool, refLat, refLon float64) (lat, lon float64) {
	const n = 1 << 17
	latF := float64(latCPR) / n
	lonF := float64(lonCPR) / n

	dLat := 360.0 / (4 * nz)
	if odd {
		dLat = 360.0 / (4*nz - 1)
	}
	j := math.Floor(refLat/dLat) + math.Floor(cprMod(refLat, dLat)/dLat-latF+0.5)
	lat = dLat * (j + latF)

	nl := cprNL(lat)
	if odd {
		nl--
	}
	ni := math.Max(nl, 1)
	dLon := 360.0 / ni
	m := math.Floor(refLon/dLon) + math.Floor(cprMod(refLon, dLon)/dLon-lonF+0.5)
	lon = dLon * (m + lonF)
	return lat, lon
}

func hypot(x, y float64) float64 { return math.Hypot(x, y) }

func headingDeg(vx, vy float64) float64 {
	h := math.Atan2(vx, vy) * 180.0 / math.Pi
	if h < 0 {
		h += 360
	}
	return h
}
