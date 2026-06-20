package server

import (
	"encoding/json"
	"net/http"

	"github.com/iamoumeng/websdr/internal/satellite"
)

// HandleMeteorCatalog returns supported satellites (legacy path).
func HandleMeteorCatalog(w http.ResponseWriter, r *http.Request) {
	HandleSatelliteCatalog(w, r)
}

// HandleSatelliteCatalog returns supported satellites and default channel sets.
func HandleSatelliteCatalog(w http.ResponseWriter, r *http.Request) {
	channels := make(map[int][]satellite.MSUChannel)
	for _, e := range satellite.SatelliteCatalog {
		entry := e
		channels[e.Norad] = satellite.ChannelsFor(&entry)
	}
	writeJSON(w, map[string]interface{}{
		"satellites": satellite.SatelliteCatalog,
		"channels":   channels,
		"msu":        satellite.MSUChannels,
	})
}

// HandleMeteorTLE returns cached TLE lines (legacy path).
func HandleMeteorTLE(w http.ResponseWriter, r *http.Request) {
	HandleSatelliteTLE(w, r)
}

// HandleSatelliteTLE returns cached TLE lines for catalog satellites.
func HandleSatelliteTLE(w http.ResponseWriter, r *http.Request) {
	tle, err := satellite.FetchTLE()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]interface{}{"tle": tle})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(v)
}
