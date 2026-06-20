package protocol

import "time"

// minTuneHz / maxTuneHz are RTL-SDR style limits (center frequency).
// True HF (e.g. 10 MHz WWV, 14 MHz 20m) is below ~24 MHz and needs an upconverter.
const (
	minTuneHz = uint64(24_000_000)
	maxTuneHz = uint64(1_766_000_000)
)

// BandReachable reports whether the tuner center is within hardware limits.
func BandReachable(b Band) bool {
	return b.CenterHz >= minTuneHz && b.CenterHz <= maxTuneHz
}

// Band is one stop on the sweep plan. Types lists the protocol categories that
// may appear in this frequency range (used for classification hints).
// Decode selects a real decoder instead of spectrum-only detection.
type Band struct {
	Name     string
	CenterHz uint64
	RateHz   uint
	Dwell    time.Duration
	Types    []string
	Decode   string // "", "adsb", "ais"
}

// Bands covers services reachable within minTuneHz..maxTuneHz.
// HF 12m (24.9 MHz) omitted — R820T is unreliable at the 24 MHz floor; use radio + Q.
var Bands = []Band{
	{Name: "10米 CW (28 MHz)", CenterHz: 28_300_000, RateHz: 2_048_000, Dwell: 3 * time.Second, Types: []string{"cw"}},
	{Name: "6米 CW (50 MHz)", CenterHz: 50_125_000, RateHz: 2_048_000, Dwell: 3 * time.Second, Types: []string{"cw"}},
	{Name: "FM 广播", CenterHz: 98_000_000, RateHz: 2_048_000, Dwell: 3 * time.Second, Types: []string{"broadcast"}},
	{Name: "气象卫星 APT", CenterHz: 137_100_000, RateHz: 1_024_000, Dwell: 3 * time.Second, Types: []string{"satellite"}},
	{Name: "ACARS", CenterHz: 131_550_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"acars"}},
	{Name: "业余 2m", CenterHz: 145_825_000, RateHz: 1_024_000, Dwell: 3 * time.Second, Types: []string{"cw", "satellite", "aprs"}},
	{Name: "VHF 寻呼/APRS", CenterHz: 150_000_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"cw", "paging", "aprs"}},
	{Name: "AIS", CenterHz: 162_000_000, RateHz: 1_536_000, Dwell: 5 * time.Second, Types: []string{"ais"}, Decode: "ais"},
	{Name: "UHF 对讲", CenterHz: 409_750_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"dmr"}},
	{Name: "70cm 业余卫星", CenterHz: 436_500_000, RateHz: 1_024_000, Dwell: 3 * time.Second, Types: []string{"satellite"}},
	{Name: "70cm 业余 CW", CenterHz: 432_200_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"cw"}},
	{Name: "TETRA", CenterHz: 385_000_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"tetra"}},
	{Name: "ISM 433", CenterHz: 433_920_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"ism", "lora"}},
	{Name: "ISM 868", CenterHz: 868_100_000, RateHz: 1_024_000, Dwell: 2 * time.Second, Types: []string{"ism", "lora"}},
	{Name: "GSM900", CenterHz: 936_400_000, RateHz: 2_048_000, Dwell: 3 * time.Second, Types: []string{"cellular"}},
	{Name: "ADS-B", CenterHz: 1_090_000_000, RateHz: 2_000_000, Dwell: 5 * time.Second, Types: []string{"adsb"}, Decode: "adsb"},
	// 1710–1766 MHz 上限内可扫到的蜂窝/L 波段（B3 下行 1805+ 超出 1766 无法覆盖）
	{Name: "蜂窝 1710–1761", CenterHz: 1_740_000_000, RateHz: 2_048_000, Dwell: 3 * time.Second, Types: []string{"cellular"}},
	{Name: "NOAA HRPT", CenterHz: 1_702_500_000, RateHz: 2_048_000, Dwell: 4 * time.Second, Types: []string{"satellite"}},
}

// WeatherAPTBandIndex returns the 137 MHz APT sweep index, or -1.
func WeatherAPTBandIndex() int {
	for i, b := range Bands {
		if b.CenterHz >= 136_500_000 && b.CenterHz <= 137_500_000 && HasType(b, "satellite") {
			return i
		}
	}
	return -1
}
