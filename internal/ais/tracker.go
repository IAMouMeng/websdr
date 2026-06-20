package ais

import (
	"sort"
	"sync"
	"time"
)

// Tracker maintains the live vessel table keyed by MMSI, merging position
// reports with static (name/type) reports. Safe for concurrent Update and
// Snapshot.

type vessel struct {
	mmsi     uint32
	name     string
	callsign string
	shipType int
	hasPos   bool
	lat, lon float64
	sog      float64
	cog      float64
	heading  int
	messages int
	lastSeen time.Time
}

// VesselState is the JSON-friendly snapshot sent to clients.
type VesselState struct {
	MMSI     uint32  `json:"mmsi"`
	Name     string  `json:"name,omitempty"`
	Callsign string  `json:"callsign,omitempty"`
	ShipType int     `json:"shipType,omitempty"`
	HasPos   bool    `json:"hasPos"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	SOG      float64 `json:"sog"`     // knots, -1 if unknown
	COG      float64 `json:"cog"`     // degrees, -1 if unknown
	Heading  int     `json:"heading"` // degrees, -1 if unknown
	Messages int     `json:"msgs"`
	Seen     float64 `json:"seen"` // seconds since last message
}

type Tracker struct {
	mu      sync.Mutex
	vessels map[uint32]*vessel
}

func NewTracker() *Tracker {
	return &Tracker{vessels: make(map[uint32]*vessel)}
}

func (t *Tracker) Update(r *Report) {
	if r == nil || r.MMSI == 0 {
		return
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	v := t.vessels[r.MMSI]
	if v == nil {
		v = &vessel{mmsi: r.MMSI, sog: -1, cog: -1, heading: -1}
		t.vessels[r.MMSI] = v
	}
	v.messages++
	v.lastSeen = now

	if r.Name != "" {
		v.name = r.Name
	}
	if r.Callsign != "" {
		v.callsign = r.Callsign
	}
	if r.ShipType != 0 {
		v.shipType = r.ShipType
	}
	if r.HasPos {
		v.hasPos = true
		v.lat = r.Lat
		v.lon = r.Lon
	}
	if r.SOG >= 0 {
		v.sog = r.SOG
	}
	if r.COG >= 0 {
		v.cog = r.COG
	}
	if r.Heading >= 0 {
		v.heading = r.Heading
	}
}

// Snapshot returns the current table sorted by MMSI, dropping vessels not heard
// from within maxAge.
func (t *Tracker) Snapshot(maxAge time.Duration) []VesselState {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]VesselState, 0, len(t.vessels))
	for mmsi, v := range t.vessels {
		age := now.Sub(v.lastSeen)
		if age > maxAge {
			delete(t.vessels, mmsi)
			continue
		}
		out = append(out, VesselState{
			MMSI:     v.mmsi,
			Name:     v.name,
			Callsign: v.callsign,
			ShipType: v.shipType,
			HasPos:   v.hasPos,
			Lat:      v.lat,
			Lon:      v.lon,
			SOG:      v.sog,
			COG:      v.cog,
			Heading:  v.heading,
			Messages: v.messages,
			Seen:     age.Seconds(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MMSI < out[j].MMSI })
	return out
}
