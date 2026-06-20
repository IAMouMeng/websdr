package protocol

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	carrierStale       = 45 * time.Second
	decodedStale       = 3 * time.Minute
	fullScanCarrierStale = 30 * time.Minute
)

// Tracker merges detections across sweep cycles and ages out stale signals.
type Tracker struct {
	mu      sync.Mutex
	entries map[string]*trackEntry
}

type trackEntry struct {
	sig      Signal
	lastSeen time.Time
	msgs     int
}

func NewTracker() *Tracker {
	return &Tracker{entries: make(map[string]*trackEntry)}
}

func (t *Tracker) Reset() {
	t.mu.Lock()
	t.entries = make(map[string]*trackEntry)
	t.mu.Unlock()
}

// Upsert inserts or updates one signal. Decoded rows are not downgraded by
// spectrum-only hits with the same id.
func (t *Tracker) Upsert(sig Signal) {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	if e, ok := t.entries[sig.ID]; ok {
		if e.sig.Decoded && !sig.Decoded {
			if sig.Strength != 0 {
				e.sig.Strength = sig.Strength
			}
		} else if e.sig.Decoded && sig.Decoded {
			keep := e.sig
			if sig.Image != "" && imageLines(sig) > imageLines(keep) {
				keep = sig
			} else if sig.Image != "" && keep.Image == "" {
				keep.Image = sig.Image
				if sig.Decode != nil {
					if keep.Decode == nil {
						keep.Decode = sig.Decode
					} else if sig.Decode.ImageLines > keep.Decode.ImageLines {
						keep.Decode.ImageLines = sig.Decode.ImageLines
					}
				}
			} else {
				keep.Strength = sig.Strength
				if sig.Image != "" && imageLines(sig) > imageLines(keep) {
					keep.Image = sig.Image
				}
				if sig.Decode != nil && keep.Decode != nil && sig.Decode.ImageLines > keep.Decode.ImageLines {
					keep.Decode.ImageLines = sig.Decode.ImageLines
				}
			}
			e.sig = keep
		} else {
			e.sig = sig
		}
		e.lastSeen = now
		e.msgs++
		if sig.Msgs > e.msgs {
			e.msgs = sig.Msgs
		}
		return
	}
	msgs := sig.Msgs
	if msgs < 1 {
		msgs = 1
	}
	t.entries[sig.ID] = &trackEntry{sig: sig, lastSeen: now, msgs: msgs}
}

// PatchAPTProgress updates line count / strength without replacing the image.
func (t *Tracker) PatchAPTProgress(id string, lines, strength int) {
	if lines <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[id]
	if !ok || !e.sig.Decoded {
		return
	}
	if strength != 0 {
		e.sig.Strength = strength
	}
	if e.sig.Decode == nil {
		e.sig.Decode = &DecodeInfo{}
	}
	if lines > e.sig.Decode.ImageLines {
		e.sig.Decode.ImageLines = lines
	}
	if e.sig.Cols == nil {
		e.sig.Cols = map[string]interface{}{}
	}
	e.sig.Cols["pass"] = fmt.Sprintf("%d 行", lines)
	e.lastSeen = time.Now()
}

// Update ingests peaks from one dwell on a band.
func (t *Tracker) Update(peaks []Peak, band Band) {
	for _, p := range peaks {
		t.Upsert(Classify(p, band))
	}
}

// Snapshot returns live signals sorted stably for the UI.
func (t *Tracker) Snapshot() []Signal {
	return t.snapshotWithStale(carrierStale, decodedStale)
}

// SnapshotForFullScan keeps carriers for the whole sweep (no mid-scan expiry).
func (t *Tracker) SnapshotForFullScan() []Signal {
	return t.snapshotWithStale(fullScanCarrierStale, fullScanCarrierStale)
}

func (t *Tracker) snapshotWithStale(carrier, decoded time.Duration) []Signal {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]Signal, 0, len(t.entries))
	for id, e := range t.entries {
		stale := carrier
		if e.sig.Decoded {
			stale = decoded
		}
		if now.Sub(e.lastSeen) > stale {
			delete(t.entries, id)
			continue
		}
		s := e.sig
		s.Seen = now.Sub(e.lastSeen).Seconds()
		if s.Seen < 0 {
			s.Seen = 0
		}
		if s.Msgs == 0 {
			s.Msgs = e.msgs
		}
		out = append(out, s)
	}
	sortSignals(out)
	return out
}

func imageLines(s Signal) int {
	if s.Decode != nil {
		return s.Decode.ImageLines
	}
	return 0
}

func sortSignals(out []Signal) {
	typeRank := map[string]int{}
	for i, name := range []string{
		"adsb", "ais", "satellite", "cellular", "acars", "aprs", "cw", "paging", "broadcast",
		"lora", "dmr", "tetra", "ism", "wifi", "bluetooth",
	} {
		typeRank[name] = i
	}
	sort.Slice(out, func(i, j int) bool {
		ti, tj := typeRank[out[i].Type], typeRank[out[j].Type]
		if ti != tj {
			return ti < tj
		}
		if out[i].FreqHz != out[j].FreqHz {
			return out[i].FreqHz < out[j].FreqHz
		}
		return out[i].ID < out[j].ID
	})
}
