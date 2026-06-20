package protocol

import (
	"fmt"

	"github.com/iamoumeng/websdr/internal/adsb"
	"github.com/iamoumeng/websdr/internal/ais"
)

// FromAircraft converts a decoded ADS-B target into a protocol-analysis row.
func FromAircraft(ac adsb.AircraftState) Signal {
	label := ac.Callsign
	if label == "" {
		label = ac.ICAO
	}
	alt := "—"
	if ac.HasAlt {
		alt = fmt.Sprintf("%d ft", ac.Altitude)
	}
	speed := "—"
	if ac.HasVel {
		speed = fmt.Sprintf("%.0f kt", ac.Speed)
	}
	strength := -58 - int(ac.Seen*2)
	if strength < -95 {
		strength = -95
	}

	details := [][2]string{
		{"ICAO", ac.ICAO},
		{"呼号", ac.Callsign},
		{"高度", alt},
		{"地速", speed},
		{"检测", "ADS-B 解码"},
	}
	if ac.HasVel {
		details = append(details, [2]string{"航向", fmt.Sprintf("%.0f°", ac.Heading)})
		details = append(details, [2]string{"爬升", fmt.Sprintf("%d ft/min", ac.VertRate)})
	}
	if ac.HasPos {
		details = append(details, [2]string{"位置", fmt.Sprintf("%.4f°N %.4f°E", ac.Lat, ac.Lon)})
	} else {
		details = append(details, [2]string{"位置", "CPR 配对中"})
	}

	return Signal{
		ID:          "adsb-" + ac.ICAO,
		Type:        "adsb",
		Label:       label,
		Freq:        "1090 MHz",
		FreqHz:      1_090_000_000,
		Strength:    strength,
		StrengthKey: "rssi",
		Decoded:     true,
		Cols: map[string]interface{}{
			"icao":     ac.ICAO,
			"callsign": ac.Callsign,
			"alt":      alt,
			"speed":    speed,
		},
		Details: details,
		Seen:    ac.Seen,
		Msgs:    ac.Messages,
	}
}

// FromVessel converts a decoded AIS target into a protocol-analysis row.
func FromVessel(v ais.VesselState) Signal {
	label := v.Name
	if label == "" {
		label = fmt.Sprintf("%09d", v.MMSI)
	}
	sog := "—"
	if v.SOG >= 0 {
		sog = fmt.Sprintf("%.1f kn", v.SOG)
	}
	cog := "—"
	if v.COG >= 0 {
		cog = fmt.Sprintf("%.0f°", v.COG)
	}
	strength := -60 - int(v.Seen*2)
	if strength < -95 {
		strength = -95
	}

	details := [][2]string{
		{"MMSI", fmt.Sprintf("%09d", v.MMSI)},
		{"船名", v.Name},
		{"呼号", v.Callsign},
		{"航速", sog},
		{"航向", cog},
		{"检测", "AIS 解码"},
	}
	if v.HasPos {
		details = append(details, [2]string{"位置", fmt.Sprintf("%.4f°N %.4f°E", v.Lat, v.Lon)})
	}

	return Signal{
		ID:          fmt.Sprintf("ais-%09d", v.MMSI),
		Type:        "ais",
		Label:       label,
		Freq:        "161.975 MHz",
		FreqHz:      161_975_000,
		Strength:    strength,
		StrengthKey: "rssi",
		Decoded:     true,
		Cols: map[string]interface{}{
			"mmsi": fmt.Sprintf("%09d", v.MMSI),
			"sog":  sog,
			"cog":  cog,
			"ch":   "A/B",
		},
		Details: details,
		Seen:    v.Seen,
		Msgs:    v.Messages,
	}
}
