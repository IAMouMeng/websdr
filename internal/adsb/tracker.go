package adsb

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Tracker maintains the live aircraft table. It pairs even/odd CPR frames into
// absolute positions and falls back to local decoding once a position is known.
// It is safe for concurrent Update (decoder goroutine) and Snapshot (broadcast
// goroutine).

type cprFrame struct {
	lat, lon int
	t        time.Time
	valid    bool
}

type aircraft struct {
	icao     uint32
	callsign string
	lat, lon float64
	hasPos   bool
	altitude int
	hasAlt   bool
	speed    float64
	heading  float64
	vertRate int
	hasVel   bool
	messages int
	lastSeen time.Time

	even, odd cprFrame
}

// AircraftState is the JSON-friendly snapshot sent to clients.
type AircraftState struct {
	ICAO     string  `json:"icao"`
	Callsign string  `json:"callsign,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	HasPos   bool    `json:"hasPos"`
	Altitude int     `json:"alt,omitempty"`
	HasAlt   bool    `json:"hasAlt"`
	Speed    float64 `json:"speed,omitempty"`
	Heading  float64 `json:"heading,omitempty"`
	VertRate int     `json:"vs,omitempty"`
	HasVel   bool    `json:"hasVel"`
	Messages int     `json:"msgs"`
	Seen     float64 `json:"seen"` // seconds since last message
}

type Tracker struct {
	mu  sync.Mutex
	acs map[uint32]*aircraft
}

func NewTracker() *Tracker {
	return &Tracker{acs: make(map[uint32]*aircraft)}
}

// cprMaxAge bounds how far apart an even/odd pair may be for global decoding.
const cprMaxAge = 10 * time.Second

func (t *Tracker) Update(m *Message) {
	if m == nil {
		return
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	a := t.acs[m.ICAO]
	if a == nil {
		a = &aircraft{icao: m.ICAO}
		t.acs[m.ICAO] = a
	}
	a.messages++
	a.lastSeen = now

	if m.Callsign != "" {
		a.callsign = m.Callsign
	}
	if m.HasAltitude {
		a.altitude = m.Altitude
		a.hasAlt = true
	}
	if m.HasVelocity {
		a.speed = m.GroundSpeed
		a.heading = m.Heading
		a.vertRate = m.VertRate
		a.hasVel = true
	}
	if m.HasPosition {
		t.updatePosition(a, m, now)
	}
}

func (t *Tracker) updatePosition(a *aircraft, m *Message, now time.Time) {
	if m.CPROdd {
		a.odd = cprFrame{m.LatCPR, m.LonCPR, now, true}
	} else {
		a.even = cprFrame{m.LatCPR, m.LonCPR, now, true}
	}

	if a.even.valid && a.odd.valid && absDur(a.even.t.Sub(a.odd.t)) < cprMaxAge {
		evenNewer := a.even.t.After(a.odd.t)
		if lat, lon, ok := cprGlobal(a.even.lat, a.even.lon, a.odd.lat, a.odd.lon, evenNewer); ok {
			a.lat, a.lon, a.hasPos = lat, lon, true
			return
		}
	}
	if a.hasPos {
		a.lat, a.lon = cprLocal(m.LatCPR, m.LonCPR, m.CPROdd, a.lat, a.lon)
	}
}

// Snapshot returns the current table sorted by ICAO, dropping aircraft not
// heard from within maxAge.
func (t *Tracker) Snapshot(maxAge time.Duration) []AircraftState {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]AircraftState, 0, len(t.acs))
	for icao, a := range t.acs {
		age := now.Sub(a.lastSeen)
		if age > maxAge {
			delete(t.acs, icao)
			continue
		}
		out = append(out, AircraftState{
			ICAO:     fmt.Sprintf("%06X", a.icao),
			Callsign: a.callsign,
			Lat:      a.lat,
			Lon:      a.lon,
			HasPos:   a.hasPos,
			Altitude: a.altitude,
			HasAlt:   a.hasAlt,
			Speed:    a.speed,
			Heading:  a.heading,
			VertRate: a.vertRate,
			HasVel:   a.hasVel,
			Messages: a.messages,
			Seen:     age.Seconds(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ICAO < out[j].ICAO })
	return out
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
