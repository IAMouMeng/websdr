package satellite

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var tleGroups = []string{"weather", "stations", "active"}

var (
	tleMu    sync.RWMutex
	tleCache map[int][2]string
	tleAt    time.Time
)

// FetchTLE downloads TLEs from CelesTrak (multiple groups + catalog NORAD fallback) and caches them.
func FetchTLE() (map[int][2]string, error) {
	tleMu.RLock()
	if tleCache != nil && time.Since(tleAt) < 6*time.Hour {
		out := copyTLE(tleCache)
		tleMu.RUnlock()
		return out, nil
	}
	tleMu.RUnlock()

	merged := make(map[int][2]string)
	client := &http.Client{Timeout: 20 * time.Second}
	for _, group := range tleGroups {
		parsed, err := fetchTLEGroup(client, group)
		if err != nil {
			continue
		}
		for k, v := range parsed {
			merged[k] = v
		}
	}
	for i := range SatelliteCatalog {
		norad := SatelliteCatalog[i].Norad
		if _, ok := merged[norad]; ok {
			continue
		}
		if lines, err := fetchTLEByNorad(client, norad); err == nil {
			merged[norad] = lines
		}
	}
	if len(merged) == 0 {
		return nil, fmt.Errorf("celestrak: no TLE data")
	}

	tleMu.Lock()
	tleCache = merged
	tleAt = time.Now()
	out := copyTLE(merged)
	tleMu.Unlock()
	return out, nil
}

// TLEForNorad returns cached or freshly fetched TLE lines for one satellite.
func TLEForNorad(norad int) ([2]string, error) {
	all, err := FetchTLE()
	if err != nil {
		return [2]string{}, err
	}
	lines, ok := all[norad]
	if !ok {
		return [2]string{}, fmt.Errorf("TLE not found for NORAD %d", norad)
	}
	return lines, nil
}

func fetchTLEGroup(client *http.Client, group string) (map[int][2]string, error) {
	url := fmt.Sprintf("https://celestrak.org/NORAD/elements/gp.php?GROUP=%s&FORMAT=tle", group)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("celestrak %s: HTTP %d", group, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseTLEText(string(body)), nil
}

func fetchTLEByNorad(client *http.Client, norad int) ([2]string, error) {
	url := fmt.Sprintf("https://celestrak.org/NORAD/elements/gp.php?CATNR=%d&FORMAT=tle", norad)
	resp, err := client.Get(url)
	if err != nil {
		return [2]string{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return [2]string{}, fmt.Errorf("celestrak CATNR %d: HTTP %d", norad, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return [2]string{}, err
	}
	parsed := parseTLEText(string(body))
	lines, ok := parsed[norad]
	if !ok {
		return [2]string{}, fmt.Errorf("TLE not found for NORAD %d", norad)
	}
	return lines, nil
}

func copyTLE(src map[int][2]string) map[int][2]string {
	out := make(map[int][2]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func parseTLEText(text string) map[int][2]string {
	lines := strings.Split(text, "\n")
	out := make(map[int][2]string)
	for i := 0; i+2 < len(lines); i++ {
		name := strings.TrimSpace(lines[i])
		l1 := strings.TrimSpace(lines[i+1])
		l2 := strings.TrimSpace(lines[i+2])
		if len(l1) < 69 || len(l2) < 69 || l1[0] != '1' || l2[0] != '2' {
			continue
		}
		noradStr := strings.TrimSpace(l1[2:7])
		var norad int
		if _, err := fmt.Sscanf(noradStr, "%d", &norad); err != nil {
			continue
		}
		_ = name
		out[norad] = [2]string{l1, l2}
		i += 2
	}
	return out
}
