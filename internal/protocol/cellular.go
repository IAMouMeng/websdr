package protocol

import "fmt"

// CellularInfo is inferred from carrier frequency without a protocol decoder.
type CellularInfo struct {
	RAT    string // GSM, LTE, NR
	Band   string // e.g. GSM900, B3, B39
	ARFCN  string // GSM ARFCN or LTE EARFCN estimate
	PCI    string // only from real decode
	Bw     string
	Note   string
}

// InferCellular maps a detected carrier frequency to GSM/LTE band metadata.
// PCI / MCC / Cell ID require gr-gsm or LTE cell search — not available here.
func InferCellular(freqHz uint64, bwHz float64) CellularInfo {
	mhz := float64(freqHz) / 1e6
	bwK := bwHz / 1000

	// GSM900 P-GSM downlink: 935.2 + 0.2*(n-1) MHz, ARFCN 1..124
	if mhz >= 921.0 && mhz <= 960.0 {
		arfcn := int((mhz-935.0)/0.2 + 1.5)
		if arfcn < 1 {
			arfcn = 1
		}
		if arfcn > 124 {
			arfcn = 124
		}
		chMHz := 935.0 + 0.2*float64(arfcn-1)
		return CellularInfo{
			RAT:   "GSM",
			Band:  "GSM900",
			ARFCN: fmt.Sprintf("ARFCN %d", arfcn),
			PCI:   "—",
			Bw:    gsmBwLabel(bwK),
			Note:  fmt.Sprintf("GSM 信道中心约 %.3f MHz；BCCH/PCI 需 gr-gsm 解码", chMHz),
		}
	}

	// GSM1800 / LTE B3 downlink: 1805.2 + 0.2*(n-512) MHz
	if mhz >= 1805.0 && mhz <= 1880.0 {
		arfcn := int((mhz-1805.0)/0.2 + 512.5)
		if arfcn < 512 {
			arfcn = 512
		}
		if arfcn > 885 {
			arfcn = 885
		}
		earfcn := 1200 + (arfcn - 512) // rough B3 DL EARFCN mapping
		return CellularInfo{
			RAT:   "LTE/GSM",
			Band:  "B3 / DCS1800",
			ARFCN: fmt.Sprintf("EARFCN ~%d", earfcn),
			PCI:   "—",
			Bw:    lteBwLabel(bwK),
			Note:  "PCI / SIB 需 LTE 小区搜索解码",
		}
	}

	// LTE B39 TDD (1880–1920 MHz)
	if mhz >= 1880.0 && mhz <= 1920.0 {
		earfcn := 38450 + int((mhz-1880.0)/0.2)
		return CellularInfo{
			RAT:   "LTE",
			Band:  "B39 TDD",
			ARFCN: fmt.Sprintf("EARFCN ~%d", earfcn),
			PCI:   "—",
			Bw:    lteBwLabel(bwK),
			Note:  "TDD 需 gr-gsm / srsRAN 类工具解 MIB/SIB",
		}
	}

	// LTE B40 / B41 lower check
	if mhz >= 2300.0 && mhz <= 2400.0 {
		return CellularInfo{
			RAT: "LTE", Band: "B40 TDD", ARFCN: "—", PCI: "—",
			Bw: lteBwLabel(bwK), Note: "超出多数 RTL-SDR 上限",
		}
	}
	if mhz >= 2515.0 && mhz <= 2675.0 {
		return CellularInfo{
			RAT: "LTE", Band: "B41 TDD", ARFCN: "—", PCI: "—",
			Bw: lteBwLabel(bwK), Note: "超出 RTL-SDR 频段",
		}
	}

	// 1710–1785 MHz: LTE B3 上行 / n77 边缘（设备上限附近）
	if mhz >= 1710.0 && mhz <= 1785.0 {
		return CellularInfo{
			RAT: "LTE/NR", Band: "B3 UL / n77 边缘", ARFCN: "—", PCI: "—",
			Bw: lteBwLabel(bwK), Note: "多为终端上行或边缘频段",
		}
	}

	return CellularInfo{
		RAT: "—", Band: "—", ARFCN: "—", PCI: "—",
		Bw: fmt.Sprintf("%.0f kHz", bwK), Note: "未识别蜂窝频段",
	}
}

func gsmBwLabel(measuredKHz float64) string {
	if measuredKHz < 50 {
		return "200 kHz (GSM 信道)"
	}
	return fmt.Sprintf("%.0f kHz", measuredKHz)
}

func lteBwLabel(measuredKHz float64) string {
	if measuredKHz < 500 {
		return "5–20 MHz (LTE 预期)"
	}
	return fmt.Sprintf("%.0f kHz", measuredKHz)
}

// SnapCellularPeaks merges narrow FFT peaks onto standard channel grids.
func SnapCellularPeaks(peaks []Peak, band Band) []Peak {
	if len(band.Types) == 0 || band.Types[0] != "cellular" {
		return peaks
	}
	merged := make(map[uint64]Peak)
	for _, p := range peaks {
	 snapped := snapCellFreq(p)
		if cur, ok := merged[snapKey(snapped.FreqHz)]; ok {
			if snapped.PowerDB > cur.PowerDB {
				cur.PowerDB = snapped.PowerDB
			}
			if snapped.BwHz > cur.BwHz {
				cur.BwHz = snapped.BwHz
			}
			merged[snapKey(snapped.FreqHz)] = cur
		} else {
			merged[snapKey(snapped.FreqHz)] = snapped
		}
	}
	out := make([]Peak, 0, len(merged))
	for _, p := range merged {
		out = append(out, p)
	}
	return out
}

func snapKey(freqHz uint64) uint64 {
	// 100 kHz grid bucket
	return (freqHz + 50_000) / 100_000
}

func snapCellFreq(p Peak) Peak {
	mhz := float64(p.FreqHz) / 1e6
	if mhz >= 921.0 && mhz <= 960.0 {
		arfcn := int((mhz-935.0)/0.2 + 1.5)
		if arfcn < 1 {
			arfcn = 1
		}
		if arfcn > 124 {
			arfcn = 124
		}
		center := uint64((935.0 + 0.2*float64(arfcn-1)) * 1e6)
		p.FreqHz = center
		if p.BwHz < 150_000 {
			p.BwHz = 200_000
		}
	}
	return p
}
