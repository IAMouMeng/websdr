package protocol

// Signal is one row in the protocol-analysis UI.
type Signal struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Label       string                 `json:"label"`
	Freq        string                 `json:"freq"`
	FreqHz      uint64                 `json:"freqHz"`
	Strength    int                    `json:"strength"`
	StrengthKey string                 `json:"strengthKey"`
	Cols        map[string]interface{} `json:"cols"`
	Details     [][2]string            `json:"details"`
	Seen        float64                `json:"seen"`
	Msgs        int                    `json:"msgs,omitempty"`
	Decoded     bool                   `json:"decoded,omitempty"`
	Decode      *DecodeInfo            `json:"decode,omitempty"`
	Image       string                 `json:"image,omitempty"`
}

// DecodeInfo holds structured demod results for the UI.
type DecodeInfo struct {
	Service    string  `json:"service,omitempty"`
	Mod        string  `json:"mod,omitempty"`
	Direction  string  `json:"direction,omitempty"`
	Subcarrier string  `json:"subcarrier,omitempty"`
	Metric     string  `json:"metric,omitempty"`
	MetricLabel string `json:"metricLabel,omitempty"`
	ImageLines  int    `json:"imageLines,omitempty"`
	Note        string `json:"note,omitempty"`
	PI          string `json:"pi,omitempty"`
	PS          string `json:"ps,omitempty"`
	PTY         string `json:"pty,omitempty"`
}

// Peak is a detected carrier from one dwell on a scan band.
type Peak struct {
	FreqHz  uint64
	PowerDB float32
	BwHz    float64
}
