package protocol

import (
	"fmt"
	"sort"
	"strings"
)

// ScanProgress is live full-scan state for the UI.
type ScanProgress struct {
	BandIdx   int     `json:"bandIdx"`
	BandTotal int     `json:"bandTotal"`
	BandName  string  `json:"bandName"`
	CenterHz  uint64  `json:"centerHz"`
	RateHz    uint    `json:"rateHz"`
	Pct       float64 `json:"pct"`
	Phase     string  `json:"phase"` // scanning | analyzing | done
}

// BandRecord stores signals detected during one dwell on a scan band.
type BandRecord struct {
	Band    Band
	Signals []Signal
}

// BandSummary is the per-band report after a full scan completes.
type BandSummary struct {
	Name         string   `json:"name"`
	CenterMHz    string   `json:"centerMHz"`
	SignalCount  int      `json:"signalCount"`
	PrimaryTypes []string `json:"primaryTypes"`
	Summary      string   `json:"summary"`
	Signals      []Signal `json:"signals,omitempty"`
}

// SignalsForBand picks tracker rows that belong to one scan stop.
func SignalsForBand(all []Signal, band Band) []Signal {
	switch band.Decode {
	case "adsb":
		return filterSignalsByType(all, "adsb")
	case "ais":
		return filterSignalsByType(all, "ais")
	default:
		lo := int64(band.CenterHz) - int64(band.RateHz)/2
		hi := int64(band.CenterHz) + int64(band.RateHz)/2
		var out []Signal
		for _, s := range all {
			f := int64(s.FreqHz)
			if f >= lo && f <= hi {
				out = append(out, s)
			}
		}
		return out
	}
}

func filterSignalsByType(all []Signal, typ string) []Signal {
	var out []Signal
	for _, s := range all {
		if s.Type == typ {
			out = append(out, s)
		}
	}
	return out
}

// SummarizeBand builds a human-readable row for one band record.
func SummarizeBand(rec BandRecord) BandSummary {
	typeCount := map[string]int{}
	for _, s := range rec.Signals {
		typeCount[s.Type]++
	}
	types := sortedTypeKeys(typeCount)

	sum := BandSummary{
		Name:         rec.Band.Name,
		CenterMHz:    fmt.Sprintf("%.3f", float64(rec.Band.CenterHz)/1e6),
		SignalCount:  len(rec.Signals),
		PrimaryTypes: types,
		Signals:      rec.Signals,
	}
	sum.Summary = bandSummaryText(rec.Band, rec.Signals, typeCount, types)
	return sum
}

// FullScanRequiresDecode reports types that should only appear after demod/decode.
func FullScanRequiresDecode(typ string) bool {
	switch typ {
	case "ism", "lora", "wifi", "bluetooth", "dmr", "tetra", "paging", "cw":
		return true
	default:
		return false
	}
}

// FullScanKeepSignal filters full-scan rows for the UI list.
func FullScanKeepSignal(sig Signal) bool {
	if !FullScanRequiresDecode(sig.Type) {
		return true
	}
	return sig.Decoded
}

// AggregateSignalsFromRecords deduplicates signals collected per chunk.
func AggregateSignalsFromRecords(records []BandRecord) []Signal {
	seen := map[string]bool{}
	var out []Signal
	for _, rec := range records {
		for _, s := range rec.Signals {
			if seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	sortSignals(out)
	return out
}

// SummarizeFullScan turns all band records into a final report (ungrouped).
func SummarizeFullScan(records []BandRecord) []BandSummary {
	return SummarizeFullScanGrouped(records, summaryGroupHz)
}

func sortedTypeKeys(counts map[string]int) []string {
	type pair struct {
		typ string
		n   int
	}
	var ps []pair
	for t, n := range counts {
		ps = append(ps, pair{t, n})
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].n != ps[j].n {
			return ps[i].n > ps[j].n
		}
		return ps[i].typ < ps[j].typ
	})
	types := make([]string, len(ps))
	for i, p := range ps {
		types[i] = p.typ
	}
	return types
}

func bandSummaryText(band Band, sigs []Signal, typeCount map[string]int, types []string) string {
	if len(sigs) == 0 {
		switch band.Decode {
		case "adsb":
			return "本段未检测到 ADS-B 飞机"
		case "ais":
			return "本段未检测到 AIS 船舶"
		default:
			return "本段未发现明显载波"
		}
	}

	if band.Decode == "adsb" {
		return fmt.Sprintf("检测到 %d 架飞机的 ADS-B 信号", len(sigs))
	}
	if band.Decode == "ais" {
		return fmt.Sprintf("检测到 %d 艘船舶的 AIS 信号", len(sigs))
	}

	decoded := 0
	for _, s := range sigs {
		if s.Decoded {
			decoded++
		}
	}
	typeLabels := make([]string, 0, len(types))
	for _, t := range types {
		typeLabels = append(typeLabels, typeLabelCN(t))
	}
	base := fmt.Sprintf("%d 个信号", len(sigs))
	if len(typeLabels) > 0 {
		base += "（" + strings.Join(typeLabels, "、") + "）"
	}
	if decoded > 0 {
		base += fmt.Sprintf("，%d 个已解调确认", decoded)
	}
	return base
}

func typeLabelCN(typ string) string {
	switch typ {
	case "adsb":
		return "ADS-B"
	case "ais":
		return "AIS"
	case "satellite":
		return "卫星"
	case "cellular":
		return "蜂窝"
	case "acars":
		return "ACARS"
	case "aprs":
		return "APRS"
	case "cw":
		return "CW"
	case "broadcast":
		return "FM广播"
	case "paging":
		return "寻呼"
	case "lora":
		return "LoRa"
	case "dmr":
		return "DMR"
	case "tetra":
		return "TETRA"
	case "ism":
		return "ISM"
	default:
		return typ
	}
}

// BandCount returns the number of stops in the sweep plan.
func BandCount() int { return len(Bands) }
