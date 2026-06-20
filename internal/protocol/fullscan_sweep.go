package protocol

import (
	"fmt"
	"time"
)

// FullScanMaxHz is the upper edge of the one-shot full sweep (1.7 GHz).
const FullScanMaxHz = uint64(1_700_000_000)

const (
	fullScanRateHz = uint(2_048_000)
	// Step equals sample rate: each stop covers ~2 MHz; adjacent chunks touch without overlap.
	fullScanStepHz = uint64(fullScanRateHz)
	// Short dwell — a few FFT averages after tuner settle is enough for peak discovery.
	fullScanDwell          = 250 * time.Millisecond
	FullScanTuneSettle     = 150 * time.Millisecond
	FullScanMinFFTFrames   = 4
	fullScanADSDwell     = 2 * time.Second
	fullScanAISDwell     = 1500 * time.Millisecond
	fullScanCWDwell      = 12 * time.Second
	summaryGroupHz       = uint64(50_000_000)
)

// fullScanStartHz is the first center used by one-shot full sweep. The R820T is
// unreliable right at the 24 MHz floor (PLL unlock); start a little higher.
const fullScanStartHz = uint64(28_000_000)

// FullSweepBands builds a contiguous ~28 MHz – 1.7 GHz sweep plan.
func FullSweepBands() []Band {
	half := uint64(fullScanRateHz / 2)
	start := fullScanStartHz
	if start < half {
		start = half
	}
	var bands []Band
	for center := start; center <= FullScanMaxHz; center += fullScanStepHz {
		b := Band{
			CenterHz: center,
			RateHz:   fullScanRateHz,
			Dwell:    fullScanDwell,
		}
		lo, hi := center-half, center+half
		b.Name = fmt.Sprintf("%.0f–%.0f MHz", float64(lo)/1e6, float64(hi)/1e6)
		b.Types = sweepTypes(lo, hi)
		b.Decode = sweepDecode(lo, hi)
		switch b.Decode {
		case "adsb":
			b.Dwell = fullScanADSDwell
		case "ais":
			b.Dwell = fullScanAISDwell
		}
		if !BandReachable(b) {
			continue
		}
		bands = append(bands, b)
	}
	return bands
}

// FullSweepBandCount returns how many dwell stops a full sweep uses.
func FullSweepBandCount() int { return len(FullSweepBands()) }

// FullSweepDuration estimates wall-clock time for one full sweep.
func FullSweepDuration() time.Duration {
	return time.Duration(FullSweepBandCount()) * fullScanDwell
}

func sweepDecode(lo, hi uint64) string {
	if rangesOverlap(lo, hi, 1_089_000_000, 1_091_000_000) {
		return "adsb"
	}
	if rangesOverlap(lo, hi, 161_800_000, 162_200_000) {
		return "ais"
	}
	return ""
}

func sweepTypes(lo, hi uint64) []string {
	seen := map[string]bool{}
	add := func(t string) {
		if !seen[t] {
			seen[t] = true
		}
	}
	if rangesOverlap(lo, hi, 28_000_000, 29_700_000) {
		add("cw")
	}
	if rangesOverlap(lo, hi, 50_000_000, 54_000_000) {
		add("cw")
	}
	if rangesOverlap(lo, hi, 87_500_000, 108_000_000) {
		add("broadcast")
	}
	if rangesOverlap(lo, hi, 130_000_000, 137_500_000) {
		add("acars")
	}
	if rangesOverlap(lo, hi, 136_000_000, 138_500_000) {
		add("satellite")
	}
	if rangesOverlap(lo, hi, 144_000_000, 146_000_000) {
		add("cw")
		add("aprs")
		add("satellite")
	}
	if rangesOverlap(lo, hi, 147_000_000, 153_000_000) {
		add("paging")
		add("aprs")
	}
	if rangesOverlap(lo, hi, 380_000_000, 470_000_000) {
		add("dmr")
		add("tetra")
		add("ism")
		add("lora")
		add("satellite")
	}
	if rangesOverlap(lo, hi, 860_000_000, 960_000_000) {
		add("cellular")
		add("ism")
	}
	if rangesOverlap(lo, hi, 1_690_000_000, 1_710_000_000) {
		add("satellite")
		add("cellular")
	}
	if rangesOverlap(lo, hi, 1_710_000_000, 1_766_000_000) {
		add("cellular")
	}
	if len(seen) == 0 {
		add("ism")
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	return out
}

func rangesOverlap(lo, hi, blo, bhi uint64) bool {
	return hi >= blo && lo <= bhi
}

// SummarizeFullScanGrouped merges chunk records into wider bins for the UI report.
func SummarizeFullScanGrouped(records []BandRecord, groupHz uint64) []BandSummary {
	if groupHz == 0 {
		groupHz = summaryGroupHz
	}
	type bucket struct {
		lo, hi     uint64
		signals    []Signal
		typeCount  map[string]int
		chunkCount int
	}
	buckets := map[uint64]*bucket{}

	for _, rec := range records {
		lo := rec.Band.CenterHz
		if rec.Band.RateHz > 0 {
			lo = rec.Band.CenterHz - uint64(rec.Band.RateHz)/2
		}
		key := (lo / groupHz) * groupHz
		b := buckets[key]
		if b == nil {
			b = &bucket{lo: key, hi: key + groupHz, typeCount: map[string]int{}}
			buckets[key] = b
		}
		b.chunkCount++
		for _, s := range rec.Signals {
			dup := false
			for _, ex := range b.signals {
				if ex.ID == s.ID {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			b.signals = append(b.signals, s)
			b.typeCount[s.Type]++
		}
	}

	keys := make([]uint64, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sortSliceU64(keys)

	out := make([]BandSummary, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		types := sortedTypeKeys(b.typeCount)
		name := fmt.Sprintf("%.0f–%.0f MHz", float64(b.lo)/1e6, float64(b.hi)/1e6)
		center := fmt.Sprintf("%.0f", float64(b.lo+b.hi/2)/1e6)
		sum := BandSummary{
			Name:         name,
			CenterMHz:    center,
			SignalCount:  len(b.signals),
			PrimaryTypes: types,
			Signals:      b.signals,
		}
		if len(b.signals) == 0 {
			sum.Summary = fmt.Sprintf("扫描 %d 段，未发现明显载波", b.chunkCount)
		} else {
			labels := make([]string, 0, len(types))
			for _, t := range types {
				labels = append(labels, typeLabelCN(t))
			}
			sum.Summary = fmt.Sprintf("%d 个信号（%s）", len(b.signals), joinLabels(labels))
		}
		out = append(out, sum)
	}

	note := BandSummary{
		Name:        "扫描说明",
		CenterMHz:   "0–1700",
		Summary:     "全频快扫 28 MHz – 1.7 GHz（每段 2 MHz 频谱一次分析）；0–24 MHz 需 Q 通道/上变频，快扫不覆盖",
		SignalCount: countAllSignals(records),
	}
	return append([]BandSummary{note}, out...)
}

func sortSliceU64(s []uint64) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func joinLabels(labels []string) string {
	if len(labels) == 0 {
		return "—"
	}
	out := labels[0]
	for i := 1; i < len(labels); i++ {
		out += "、" + labels[i]
	}
	return out
}

func countAllSignals(records []BandRecord) int {
	n := 0
	for _, r := range records {
		n += len(r.Signals)
	}
	return n
}
