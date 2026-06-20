package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iamoumeng/websdr/internal/receiver"
)

const (
	msgSpectrum = 0x01
	msgAudio    = 0x02
	msgIQ       = 0x03
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// message is a queued outbound frame for a single client.
type message struct {
	mt   int
	data []byte
}

// client owns one websocket connection. All writes go through its send channel
// and a single writer goroutine, since gorilla/websocket forbids concurrent
// writes to the same connection.
type client struct {
	conn *websocket.Conn
	send chan message
}

func (c *client) writeLoop() {
	for msg := range c.send {
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteMessage(msg.mt, msg.data); err != nil {
			c.conn.Close() // unblock the reader; HandleWS cleans up
			return
		}
	}
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	rx      *receiver.Receiver
}

func NewHub(rx *receiver.Receiver) *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		rx:      rx,
	}
}

func (h *Hub) statusJSON() []byte {
	cfg := h.rx.Config()
	status, _ := json.Marshal(map[string]interface{}{
		"type":       "status",
		"centerFreq": cfg.CenterFreq,
		"tuneFreq":   cfg.TuneFreq,
		"sampleRate": cfg.SampleRate,
		"filterBW":   cfg.FilterBW,
		"cwPitch":    cfg.CWPitch,
		"gain":       cfg.Gain,
		"agc":        cfg.AGC,
		"mode":       cfg.Mode,
		"nr":         cfg.NR,
		"nrLevel":    cfg.NRLevel,
		"service":        cfg.Service,
		"enabled":        h.rx.Enabled(),
		"directSampling": cfg.DirectSampling,
	})
	return status
}

// broadcast queues a frame to every client, dropping it for any client whose
// send buffer is full (a slow consumer must not stall the others).
func (h *Hub) broadcast(mt int, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- message{mt, data}:
		default:
		}
	}
}

func (h *Hub) BroadcastStatus() {
	h.broadcast(websocket.TextMessage, h.statusJSON())
}

func (h *Hub) RunBroadcast(ctx context.Context) {
	go h.runAudioBroadcast(ctx)
	go h.runSpectrumBroadcast(ctx)
	go h.runIQBroadcast(ctx)
	go h.runDecodeBroadcast(ctx)
	<-ctx.Done()
}

// runDecodeBroadcast forwards digital-service decode snapshots (ADS-B aircraft,
// AIS vessels) to all clients as JSON text frames.
func (h *Hub) runDecodeBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-h.rx.DecodeChan():
			var payload []byte
			switch frame.Service {
			case receiver.ServiceADSB:
				payload, _ = json.Marshal(map[string]interface{}{
					"type": "adsb", "aircraft": frame.Aircraft,
				})
			case receiver.ServiceAIS:
				payload, _ = json.Marshal(map[string]interface{}{
					"type": "ais", "vessels": frame.Vessels,
				})
			case receiver.ServiceProtocol:
				m := map[string]interface{}{
					"type": "protocol", "signals": frame.Signals, "scanBand": frame.ScanBand,
				}
				if frame.ScanProgress != nil {
					m["scanProgress"] = frame.ScanProgress
				}
				if frame.FullScanComplete {
					m["fullScanComplete"] = true
				}
				if len(frame.BandSummaries) > 0 {
					m["bandSummaries"] = frame.BandSummaries
				}
				payload, _ = json.Marshal(m)
			case receiver.ServiceAPT:
				payload, _ = json.Marshal(map[string]interface{}{
					"type": "apt", "apt": frame.APT,
				})
			case receiver.ServiceLRPT:
				payload, _ = json.Marshal(map[string]interface{}{
					"type": "lrpt", "lrpt": frame.LRPT,
				})
			case receiver.ServiceMeteor:
				payload, _ = json.Marshal(map[string]interface{}{
					"type": "meteor", "meteor": frame.Meteor,
				})
			}
			if payload != nil {
				h.broadcast(websocket.TextMessage, payload)
			}
		}
	}
}

func (h *Hub) runAudioBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-h.rx.AudioChan():
			buf := make([]byte, 1+2+len(frame.PCM)*2)
			buf[0] = msgAudio
			binary.LittleEndian.PutUint16(buf[1:3], uint16(frame.Rate))
			for i, s := range frame.PCM {
				binary.LittleEndian.PutUint16(buf[3+i*2:], uint16(s))
			}
			h.broadcast(websocket.BinaryMessage, buf)
		}
	}
}

func (h *Hub) runSpectrumBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-h.rx.SpectrumChan():
			buf := make([]byte, 1+4+2+len(frame.Data))
			buf[0] = msgSpectrum
			binary.LittleEndian.PutUint32(buf[1:5], uint32(frame.CenterFreq))
			binary.LittleEndian.PutUint16(buf[5:7], uint16(len(frame.Data)))
			copy(buf[7:], frame.Data)
			h.broadcast(websocket.BinaryMessage, buf)
		}
	}
}

func (h *Hub) runIQBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-h.rx.IQChan():
			n := len(frame.Data)
			buf := make([]byte, 1+4+4+1+4+n)
			buf[0] = msgIQ
			binary.LittleEndian.PutUint32(buf[1:5], uint32(frame.CenterHz))
			binary.LittleEndian.PutUint32(buf[5:9], uint32(frame.Rate))
			buf[9] = frame.Channels
			binary.LittleEndian.PutUint32(buf[10:14], uint32(n))
			copy(buf[14:], frame.Data)
			h.broadcast(websocket.BinaryMessage, buf)
		}
	}
}

type clientCmd struct {
	Cmd            string  `json:"cmd"`
	Freq           uint64  `json:"freq"`
	Mode           string  `json:"mode"`
	Gain           float32 `json:"gain"`
	AGC            *bool   `json:"agc"`
	SampleRate     uint    `json:"sampleRate"`
	FilterBW       float64 `json:"filterBW"`
	CWPitch        float64 `json:"cwPitch"`
	NR             *bool   `json:"nr"`
	NRLevel        float64 `json:"nrLevel"`
	Service        string  `json:"service"`
	ProtocolListen   *bool `json:"protocolListen"`
	ProtocolFullScan *bool `json:"protocolFullScan"`
	APTListen      *bool   `json:"aptListen"`
	LRPTListen     *bool   `json:"lrptListen"`
	MeteorListen   *bool   `json:"meteorListen"`
	MeteorNorad    int     `json:"meteorNorad"`
	MeteorAutoDop  *bool   `json:"meteorAutoDoppler"`
	MeteorDoppler  float64 `json:"meteorDoppler"`
	MeteorElev     float64 `json:"meteorElevation"`
	MeteorAz       float64 `json:"meteorAzimuth"`
	MeteorRecord   *bool   `json:"meteorRecord"`
	MeteorRecPause *bool   `json:"meteorRecordPause"`
	MeteorRecCh    int     `json:"meteorRecordChannels"`
	Enabled        *bool   `json:"enabled"`
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade: %v", err)
		return
	}

	c := &client{conn: conn, send: make(chan message, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	go c.writeLoop()

	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		close(c.send)
		conn.Close()
	}()

	c.send <- message{websocket.TextMessage, h.statusJSON()}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var cmd clientCmd
		if err := json.Unmarshal(msg, &cmd); err != nil {
			continue
		}
		h.handleCmd(cmd)
	}
}

func (h *Hub) handleCmd(cmd clientCmd) {
	changed := false
	switch cmd.Cmd {
	case "tune":
		if cmd.Freq > 0 {
			h.rx.SetTuneFreq(cmd.Freq)
		}
	case "center":
		if cmd.Freq > 0 {
			h.rx.SetCenterFreq(cmd.Freq)
		}
	case "mode":
		if cmd.Mode != "" {
			h.rx.SetMode(receiver.Mode(cmd.Mode))
			changed = true
		}
	case "gain":
		h.rx.SetGain(cmd.Gain)
		changed = true
	case "agc":
		if cmd.AGC != nil {
			h.rx.SetAGC(*cmd.AGC)
			changed = true
		}
	case "bandwidth", "sampleRate":
		if cmd.SampleRate > 0 {
			h.rx.SetSampleRate(cmd.SampleRate)
			changed = true
		}
	case "filter":
		if cmd.FilterBW > 0 {
			h.rx.SetFilterBW(cmd.FilterBW)
			changed = true
		}
	case "cwpitch":
		if cmd.CWPitch > 0 {
			h.rx.SetCWPitch(cmd.CWPitch)
			changed = true
		}
	case "nr":
		if cmd.NR != nil {
			h.rx.SetNR(*cmd.NR)
			changed = true
		}
	case "nrlevel":
		if cmd.NRLevel > 0 {
			h.rx.SetNRLevel(cmd.NRLevel)
			changed = true
		}
	case "service":
		if cmd.Service != "" {
			h.rx.SetService(receiver.Service(cmd.Service))
			changed = true
		}
	case "protocolListen":
		if cmd.ProtocolListen != nil {
			h.rx.SetProtocolListen(*cmd.ProtocolListen)
			changed = true
		}
	case "protocolFullScan":
		if cmd.ProtocolFullScan != nil {
			h.rx.SetProtocolFullScan(*cmd.ProtocolFullScan)
			changed = true
		}
	case "aptListen":
		if cmd.APTListen != nil {
			on := *cmd.APTListen
			if on {
				h.rx.SetService(receiver.ServiceAPT)
			}
			h.rx.SetAPTListen(on, cmd.Freq)
			changed = true
		}
	case "lrptListen":
		if cmd.LRPTListen != nil {
			on := *cmd.LRPTListen
			if on {
				h.rx.SetService(receiver.ServiceLRPT)
			}
			h.rx.SetLRPTListen(on, cmd.Freq)
			changed = true
		}
	case "meteorListen":
		if cmd.MeteorListen != nil {
			on := *cmd.MeteorListen
			if on {
				h.rx.SetService(receiver.ServiceMeteor)
			}
			autoDop := cmd.MeteorAutoDop != nil && *cmd.MeteorAutoDop
			h.rx.SetMeteorListen(on, cmd.Freq, cmd.MeteorNorad, autoDop)
			changed = true
		}
	case "meteorTrack":
		h.rx.SetMeteorTrack(cmd.MeteorDoppler, cmd.MeteorElev, cmd.MeteorAz)
	case "meteorTune":
		if cmd.Freq > 0 {
			h.rx.SetMeteorManualTune(cmd.Freq)
		}
	case "meteorRecord":
		if cmd.MeteorRecord != nil || cmd.MeteorRecPause != nil {
			active := false
			paused := false
			if cmd.MeteorRecord != nil {
				active = *cmd.MeteorRecord
			}
			if cmd.MeteorRecPause != nil {
				paused = *cmd.MeteorRecPause
			}
			ch := cmd.MeteorRecCh
			if ch == 0 {
				ch = 2
			}
			h.rx.SetMeteorRecord(active, paused, ch)
		}
	case "receiver":
		if cmd.Enabled != nil {
			if err := h.rx.SetEnabled(*cmd.Enabled); err != nil {
				log.Printf("set receiver enabled: %v", err)
			}
			changed = true
		}
	}
	if changed {
		h.BroadcastStatus()
	}
}
