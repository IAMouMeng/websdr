package satellite

// OrbitType classifies satellite orbits for pass prediction UI.
type OrbitType string

const (
	OrbitLEO OrbitType = "leo"
	OrbitGEO OrbitType = "geo"
)

// Entry describes a supported weather / imaging satellite downlink.
type Entry struct {
	Norad      int       `json:"norad"`
	Name       string    `json:"name"`
	Downlink   uint64    `json:"downlinkHz"`
	CenterHz   uint64    `json:"centerHz"`
	SampleRate uint      `json:"sampleRate"`
	SymbolRate float64   `json:"symbolRate"`
	Modulation string    `json:"modulation"` // LRPT, LRIT
	Orbit      OrbitType `json:"orbit"`
	TLEGroup   string    `json:"tleGroup"`
}

// SatelliteCatalog lists all supported satellites (SatDump-style, not Meteor-only).
var SatelliteCatalog = []Entry{
	{Norad: 40069, Name: "Meteor-M2", Downlink: 137_900_000, CenterHz: 137_900_000, SampleRate: 2_048_000, SymbolRate: 72000, Modulation: "LRPT", Orbit: OrbitLEO, TLEGroup: "weather"},
	{Norad: 44387, Name: "Meteor-M2-2", Downlink: 137_900_000, CenterHz: 137_900_000, SampleRate: 2_048_000, SymbolRate: 72000, Modulation: "LRPT", Orbit: OrbitLEO, TLEGroup: "weather"},
	{Norad: 57190, Name: "Meteor-M2-3", Downlink: 137_900_000, CenterHz: 137_900_000, SampleRate: 2_048_000, SymbolRate: 80000, Modulation: "LRPT", Orbit: OrbitLEO, TLEGroup: "weather"},
	{Norad: 59051, Name: "Meteor-M2-4", Downlink: 137_900_000, CenterHz: 137_900_000, SampleRate: 2_048_000, SymbolRate: 80000, Modulation: "LRPT", Orbit: OrbitLEO, TLEGroup: "weather"},
	{Norad: 43823, Name: "GK-2A", Downlink: 1_691_000_000, CenterHz: 1_691_000_000, SampleRate: 2_048_000, SymbolRate: 2_000_000, Modulation: "LRIT", Orbit: OrbitGEO, TLEGroup: "weather"},
}

// MeteorCatalog is kept for backward-compatible imports.
var MeteorCatalog = SatelliteCatalog

// MSUChannel is one imaging channel shown in the decoder UI.
type MSUChannel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Band string `json:"band"`
}

// MSUChannels lists Meteor-M MSU-MR sensor channels.
var MSUChannels = []MSUChannel{
	{ID: 1, Name: "可见光", Band: "0.46–0.57 µm"},
	{ID: 2, Name: "近红外", Band: "0.52–0.58 µm"},
	{ID: 3, Name: "短波红外", Band: "0.58–0.68 µm"},
	{ID: 4, Name: "中红外", Band: "10.5–11.5 µm"},
	{ID: 5, Name: "热红外 1", Band: "11.5–12.5 µm"},
	{ID: 6, Name: "热红外 2", Band: "8.9–9.4 µm"},
}

// GK2AChannels lists GK-2A VISSR imaging channels (LRIT).
var GK2AChannels = []MSUChannel{
	{ID: 1, Name: "可见光", Band: "0.47–0.51 µm"},
	{ID: 2, Name: "近红外", Band: "0.51–0.57 µm"},
	{ID: 3, Name: "短波红外", Band: "0.64–0.70 µm"},
	{ID: 4, Name: "水汽", Band: "6.25–7.10 µm"},
	{ID: 5, Name: "长波红外", Band: "10.3–11.3 µm"},
	{ID: 6, Name: "长波红外 2", Band: "11.5–12.5 µm"},
}

// Lookup returns a catalog entry by NORAD id, or nil.
func Lookup(norad int) *Entry {
	for i := range SatelliteCatalog {
		if SatelliteCatalog[i].Norad == norad {
			return &SatelliteCatalog[i]
		}
	}
	return nil
}

// ChannelsFor returns imaging channels for a catalog entry.
func ChannelsFor(entry *Entry) []MSUChannel {
	if entry == nil {
		return MSUChannels
	}
	if entry.Norad == 43823 {
		return GK2AChannels
	}
	return MSUChannels
}

// SymbolRateLocked reports whether an estimated symbol rate indicates lock.
func SymbolRateLocked(entry *Entry, symRate float64) bool {
	if symRate <= 0 {
		return false
	}
	if entry != nil && entry.Modulation == "LRIT" {
		return symRate >= 500_000 && symRate <= 3_500_000
	}
	return symRate >= 50_000 && symRate <= 120_000
}
