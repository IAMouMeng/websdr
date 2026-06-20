package protocol

import "fmt"

// Active amateur satellite FM downlinks (Hz). Pass windows are short; scan only
// confirms carrier presence on/near these channels.
var amateurSatDownlinks = []struct {
	Name string
	Freq uint64
}{
	{"ISS", 145_800_000},
	{"AO-92", 145_880_000},
	{"AO-91", 145_960_000},
	{"SO-50", 436_795_000},
	{"PO-101", 436_510_000},
	{"RS-44", 435_610_000},
	{"SO-121", 436_666_000},
}

const (
	amsatSnapHz     = uint64(25_000)
	amsatMinBwHz    = float64(3_000)
	amsatMaxBwHz    = float64(25_000)
	amsatMinPowerDB = float32(-82)
)

// SnapAmateurSat maps freq onto the nearest catalog downlink within amsatSnapHz.
// Returns the snapped frequency and satellite name, or ("", "") if none match.
func SnapAmateurSat(freqHz uint64) (uint64, string) {
	best := freqHz
	bestDist := amsatSnapHz + 1
	name := ""
	for _, sat := range amateurSatDownlinks {
		var dist uint64
		if freqHz >= sat.Freq {
			dist = freqHz - sat.Freq
		} else {
			dist = sat.Freq - freqHz
		}
		if dist < bestDist {
			bestDist = dist
			best = sat.Freq
			name = sat.Name
		}
	}
	if bestDist <= amsatSnapHz {
		return best, name
	}
	return freqHz, ""
}

// IsNearAmateurSat reports whether freq is on a known amateur sat downlink.
func IsNearAmateurSat(freqHz uint64) bool {
	_, name := SnapAmateurSat(freqHz)
	return name != ""
}

// IsAmateurSatBand is the VHF/UHF range where FM amateur satellites operate.
func IsAmateurSatBand(freqHz uint64) bool {
	fMHz := float64(freqHz) / 1e6
	return (fMHz >= 144 && fMHz <= 147) || (fMHz >= 435 && fMHz <= 438)
}

// TryAmateurSatellite confirms a narrow-FM carrier on a known SO-/ISS downlink.
func TryAmateurSatellite(peak Peak) (DecodeResult, bool) {
	freqHz, name := SnapAmateurSat(peak.FreqHz)
	if name == "" {
		return DecodeResult{}, false
	}
	if peak.BwHz < amsatMinBwHz || peak.BwHz > amsatMaxBwHz {
		return DecodeResult{}, false
	}
	if peak.PowerDB < amsatMinPowerDB {
		return DecodeResult{}, false
	}

	fMHz := float64(freqHz) / 1e6
	bwLabel := fmt.Sprintf("%.1f kHz", peak.BwHz/1000)
	return DecodeResult{
		OK:      true,
		Label:   fmt.Sprintf("%s %.3f MHz", name, fMHz),
		Mod:     "NFM",
		Service: "业余卫星",
		Cols: map[string]interface{}{
			"svc": name + " FM", "dir": "卫星下行", "mod": "NFM", "pass": "—",
			"sub": bwLabel,
		},
		Decode: &DecodeInfo{
			Service:     name,
			Mod:         "NFM",
			Direction:   "卫星下行",
			Metric:      bwLabel,
			MetricLabel: "估计带宽",
			Note:        "业余 FM 中继/遥测；过顶时间很短，可进入无线电页 NFM 收听",
		},
		Details: [][2]string{
			{"卫星", name},
			{"频率", fmt.Sprintf("%.3f MHz", fMHz)},
			{"调制", "NFM (业余卫星)"},
			{"带宽", bwLabel},
			{"强度", fmt.Sprintf("%.0f dBm", peak.PowerDB)},
			{"检测", "载波已确认"},
		},
	}, true
}
