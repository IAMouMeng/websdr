package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/iamoumeng/websdr/internal/receiver"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

type webRTCOfferRequest struct {
	SDP string `json:"sdp"`
}

type webRTCOfferResponse struct {
	SDP string `json:"sdp"`
}

type webRTCSession struct {
	hub        *Hub
	track      *webrtc.TrackLocalStaticRTP
	packetizer rtp.Packetizer
	mu         sync.Mutex
	closed     bool
}

func (h *Hub) dispatchWebRTC(frame receiver.AudioFrame) {
	h.webrtcMu.RLock()
	sessions := make([]*webRTCSession, 0, len(h.webrtcSessions))
	for s := range h.webrtcSessions {
		sessions = append(sessions, s)
	}
	h.webrtcMu.RUnlock()

	if len(sessions) == 0 {
		return
	}

	payload := pcm48ToMulaw8k(frame.PCM)
	if len(payload) == 0 {
		return
	}

	for _, s := range sessions {
		s.writeMulaw(payload)
	}
}

func (s *webRTCSession) writeMulaw(payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.track == nil {
		return
	}

	const frameSamples = 160 // 20 ms @ 8 kHz
	for off := 0; off+frameSamples <= len(payload); off += frameSamples {
		chunk := payload[off : off+frameSamples]
		packets := s.packetizer.Packetize(chunk, frameSamples)
		for _, pkt := range packets {
			if err := s.track.WriteRTP(pkt); err != nil {
				return
			}
		}
	}
}

func (s *webRTCSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.hub.webrtcMu.Lock()
	delete(s.hub.webrtcSessions, s)
	s.hub.webrtcMu.Unlock()
	s.track = nil
}

func (h *Hub) HandleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var req webRTCOfferRequest
	if err := json.Unmarshal(body, &req); err != nil || req.SDP == "" {
		http.Error(w, "invalid offer", http.StatusBadRequest)
		return
	}

	answer, sess, err := h.createWebRTCSession(req.SDP)
	if err != nil {
		log.Printf("webrtc offer: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.webrtcMu.Lock()
	h.webrtcSessions[sess] = struct{}{}
	h.webrtcMu.Unlock()

	resp, _ := json.Marshal(webRTCOfferResponse{SDP: answer})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp)
}

func (h *Hub) createWebRTCSession(offerSDP string) (string, *webRTCSession, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return "", nil, err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return "", nil, err
	}

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
		"audio",
		"websdr",
	)
	if err != nil {
		_ = pc.Close()
		return "", nil, err
	}

	if _, err = pc.AddTrack(track); err != nil {
		_ = pc.Close()
		return "", nil, err
	}

	sess := &webRTCSession{
		hub:        h,
		track:      track,
		packetizer: rtp.NewPacketizer(160, 0, 0, &codecs.G711Payloader{}, rtp.NewRandomSequencer(), 8000),
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed ||
			state == webrtc.PeerConnectionStateDisconnected {
			sess.close()
			_ = pc.Close()
		}
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}); err != nil {
		_ = pc.Close()
		return "", nil, err
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return "", nil, err
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return "", nil, err
	}

	select {
	case <-gatherComplete:
	case <-time.After(5 * time.Second):
	}

	return pc.LocalDescription().SDP, sess, nil
}

// pcm48ToMulaw8k decimates 48 kHz int16 PCM to 8 kHz G.711 μ-law.
func pcm48ToMulaw8k(pcm []int16) []byte {
	const ratio = 6
	out := make([]byte, 0, len(pcm)/ratio)
	for i := 0; i+ratio <= len(pcm); i += ratio {
		var sum int32
		for j := 0; j < ratio; j++ {
			sum += int32(pcm[i+j])
		}
		out = append(out, linearToMulaw(int16(sum/int32(ratio))))
	}
	return out
}

func linearToMulaw(sample int16) byte {
	const mulawMax = 0x1FFF
	const mulawBias = 0x84

	sign := byte(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
		if sample < 0 {
			sample = mulawMax
		}
	}

	sample = sample + mulawBias
	if sample > mulawMax {
		sample = mulawMax
	}

	exponent := byte(7)
	for expMask := int16(0x4000); exponent > 0 && (sample&expMask) == 0; exponent-- {
		expMask >>= 1
	}

 mantissa := byte((sample >> (exponent + 3)) & 0x0F)
	return ^(sign | (exponent << 4) | mantissa)
}
