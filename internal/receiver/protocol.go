package receiver

import (
	"log"
	"time"

	"hz.tools/rf"
	"hz.tools/sdr"

	"github.com/iamoumeng/websdr/internal/adsb"
	"github.com/iamoumeng/websdr/internal/ais"
	"github.com/iamoumeng/websdr/internal/dsp"
	"github.com/iamoumeng/websdr/internal/protocol"
)

const (
	protocolSampleRate = 2048000
	protocolEmitMin    = time.Second
	fullScanEmitMin    = 200 * time.Millisecond
	maxDwellIQ         = 131072
	maxCWDwellIQ       = 8192000 // ~4 s at 2.048 MHz for keying analysis
)

// protocolScanState lives on the read goroutine only.
type protocolScanState struct {
	listening   bool
	fullScan    bool
	analyzing   bool
	bandIdx     int
	bandStart   time.Time
	accum       []float32
	accumFrames int
	dwellIQ     []complex128
	cwSpectra   [][]float32
	tracker     *protocol.Tracker
	bandRecords []protocol.BandRecord
	sweep         []protocol.Band
	lastEmit      time.Time
	lastSpecEmit  time.Time
	retunePending bool // set only while a full device reopen is in flight
}

func (s *protocolScanState) bands() []protocol.Band {
	if s.fullScan && len(s.sweep) > 0 {
		return s.sweep
	}
	return protocol.Bands
}

func (r *Receiver) initProtocolScan() {
	r.protoScan = protocolScanState{
		tracker: protocol.NewTracker(),
		accum:   make([]float32, dsp.FFTSize/2),
		dwellIQ: make([]complex128, 0, maxDwellIQ),
	}
}

func (r *Receiver) resetProtocolScan() {
	listening := r.protoScan.listening
	fullScan := r.protoScan.fullScan
	r.protoScan = protocolScanState{
		listening: listening,
		fullScan:  fullScan,
		tracker:   protocol.NewTracker(),
		accum:     make([]float32, dsp.FFTSize/2),
		dwellIQ:   make([]complex128, 0, maxDwellIQ),
	}
	if listening {
		r.protoScan.bandIdx = 0
		r.protoScan.bandStart = time.Now()
		r.tuneProtocolBand(0)
	}
}

func (r *Receiver) appendDwellIQ(iq []complex128, band protocol.Band) {
	limit := maxDwellIQ
	if protocol.NeedsCWAnalysis(band) {
		limit = maxCWDwellIQ
	}
	r.protoScan.dwellIQ = append(r.protoScan.dwellIQ, iq...)
	if len(r.protoScan.dwellIQ) > limit {
		r.protoScan.dwellIQ = r.protoScan.dwellIQ[len(r.protoScan.dwellIQ)-limit:]
	}
}

func (r *Receiver) resetProtocolDecoders() {
	r.adsbTracker = adsb.NewTracker()
	r.aisTracker = ais.NewTracker()
	r.adsbDemod.Reset()
	for _, ch := range r.aisChannels {
		ch.mixPhase = 0
		ch.filt.Reset()
		ch.demod.Reset()
	}
}

func (r *Receiver) protocolBandTuning() (center uint64, rate uint) {
	bands := r.protoScan.bands()
	if len(bands) == 0 {
		return effectiveTuning(r.Config())
	}
	idx := r.protoScan.bandIdx
	if idx < 0 || idx >= len(bands) {
		idx = 0
	}
	b := bands[idx]
	return b.CenterHz, b.RateHz
}

func (r *Receiver) prepareProtocolBands(fullScan bool) {
	if fullScan {
		r.protoScan.sweep = protocol.FullSweepBands()
	} else {
		r.protoScan.sweep = nil
	}
	r.protoScan.bandIdx = 0
}

func (r *Receiver) switchToProtocolService(fullScan bool) {
	r.mu.Lock()
	prev := r.config.Service
	r.config.Service = ServiceProtocol
	r.prepareProtocolBands(fullScan)
	r.mu.Unlock()

	r.enqueue(func() {
		r.applyServiceHardware(ServiceProtocol, prev)
		r.beginProtocolScan(fullScan)
	})
}

func (r *Receiver) SetProtocolListen(on bool) {
	if on {
		r.startProtocolScan(false)
		return
	}
	r.enqueue(func() {
		r.protoScan.listening = false
		r.protoScan.fullScan = false
		r.protoScan.analyzing = false
	})
	r.switchToRadioFromProtocol()
}

func (r *Receiver) SetProtocolFullScan(on bool) {
	if on {
		r.startProtocolScan(true)
		return
	}
	r.enqueue(func() {
		r.protoScan.listening = false
		r.protoScan.fullScan = false
		r.protoScan.analyzing = false
	})
	r.switchToRadioFromProtocol()
}

func (r *Receiver) startProtocolScan(fullScan bool) {
	r.switchToProtocolService(fullScan)
}

func (r *Receiver) switchToRadioFromProtocol() {
	r.mu.Lock()
	prev := r.config.Service
	if prev == ServiceRadio {
		r.mu.Unlock()
		return
	}
	r.config.Service = ServiceRadio
	if prev != "" {
		r.config.CenterFreq = r.config.TuneFreq
		r.syncDirectSamplingLocked(r.config.TuneFreq)
	}
	r.mu.Unlock()

	r.enqueue(func() {
		r.applyServiceHardware(ServiceRadio, prev)
		r.resetDSP()
		r.resetDecoders()
		r.lastDecodeEmit = time.Time{}
	})
}

func (r *Receiver) beginProtocolScan(fullScan bool) {
	r.protoScan.retunePending = false
	r.protoScan.listening = true
	r.protoScan.fullScan = fullScan
	r.protoScan.analyzing = false
	r.protoScan.bandIdx = 0
	r.protoScan.bandStart = time.Now()
	r.protoScan.accumFrames = 0
	r.protoScan.bandRecords = nil
	if fullScan {
		r.protoScan.sweep = protocol.FullSweepBands()
	} else {
		r.protoScan.sweep = nil
	}
	for i := range r.protoScan.accum {
		r.protoScan.accum[i] = 0
	}
	r.protoScan.dwellIQ = r.protoScan.dwellIQ[:0]
	r.protoScan.tracker.Reset()
	r.protoScan.lastEmit = time.Time{}
	r.resetProtocolDecoders()
	r.resetSpecSmooth()
	// Hardware was tuned on reopen; only retune if config/hardware diverged.
	bands := r.protoScan.bands()
	if len(bands) > 0 {
		r.mu.Lock()
		already := r.config.CenterFreq == bands[0].CenterHz &&
			r.config.SampleRate == bands[0].RateHz
		r.mu.Unlock()
		if !already {
			r.tuneProtocolBand(0)
		}
	}
}

func (r *Receiver) resetSpecSmooth() {
	for i := range r.specSmooth {
		r.specSmooth[i] = specDBMin
	}
}

func (r *Receiver) emitProtocolMonitorAudio(iq []complex128, band protocol.Band) {
	// Full-scan uses spectrum only; WFM demod per IQ block wedges the USB stream.
	if len(iq) < 4096 || r.protoScan.fullScan {
		return
	}
	r.processAudio(iq, Config{
		CenterFreq: band.CenterHz,
		SampleRate: band.RateHz,
		TuneFreq:   band.CenterHz,
		FilterBW:   200_000,
		Mode:       ModeWFM,
	}, float64(band.RateHz))
}

func (r *Receiver) emitProtocolSpectrum(iq []complex128, band protocol.Band) {
	if !r.protoScan.listening || len(iq) < dsp.FFTSize {
		return
	}
	if !r.protoScan.fullScan {
		return
	}
	now := time.Now()
	if !r.protoScan.lastSpecEmit.IsZero() && now.Sub(r.protoScan.lastSpecEmit) < 100*time.Millisecond {
		return
	}
	r.protoScan.lastSpecEmit = now
	r.emitSpectrum(iq, Config{
		CenterFreq: band.CenterHz,
		SampleRate: band.RateHz,
		TuneFreq:   band.CenterHz,
	})
}

func (r *Receiver) tuneProtocolBand(idx int) {
	bands := r.protoScan.bands()
	if idx < 0 || idx >= len(bands) {
		return
	}
	b := bands[idx]
	r.mu.Lock()
	oldCenter := r.config.CenterFreq
	oldRate := r.config.SampleRate
	r.mu.Unlock()

	if oldCenter == b.CenterHz && oldRate == b.RateHz {
		return
	}

	r.protoScan.bandIdx = idx

	// Retune inline on the read goroutine. Must not enqueue here — advanceProtocolBand
	// calls us from processLoop and enqueue would block forever waiting for drainCommands.
	r.applyProtocolBandTune(b.CenterHz, b.RateHz)
	r.protoScan.bandStart = time.Now()
}

// applyProtocolBandTune retunes on the read goroutine without stopping USB streaming.
func (r *Receiver) applyProtocolBandTune(center uint64, rate uint) {
	if r.sdr == nil {
		return
	}
	r.forceNormalTunerModeIfNeeded()
	r.mu.Lock()
	oldRate := r.config.SampleRate
	r.config.CenterFreq = center
	r.config.SampleRate = rate
	r.mu.Unlock()
	if rate > 0 && rate != oldRate {
		if err := r.sdr.SetSampleRate(rate); err != nil {
			log.Printf("protocol sample rate: %v", err)
		}
	}
	if center > 0 {
		if err := r.sdr.SetCenterFrequency(rf.Hz(center)); err != nil {
			log.Printf("protocol frequency: %v", err)
		}
	}
	r.resetSpecSmooth()
}

func (r *Receiver) processProtocol(samples sdr.SamplesU8) {
	if !r.protoScan.listening || r.protoScan.retunePending {
		return
	}

	bands := r.protoScan.bands()
	if r.protoScan.bandIdx < 0 || r.protoScan.bandIdx >= len(bands) {
		return
	}
	band := bands[r.protoScan.bandIdx]
	now := time.Now()

	switch band.Decode {
	case "adsb":
		r.feedADSB(samples)
		if n := dsp.SamplesU8ToComplex(samples, r.iqBuf); n >= dsp.FFTSize {
			r.emitProtocolSpectrum(r.iqBuf[:n], band)
			r.emitProtocolMonitorAudio(r.iqBuf[:n], band)
		}
	case "ais":
		n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
		if n > 0 {
			r.feedAIS(r.iqBuf[:n], float64(band.RateHz))
			if n >= dsp.FFTSize {
				r.emitProtocolSpectrum(r.iqBuf[:n], band)
			}
			r.emitProtocolMonitorAudio(r.iqBuf[:n], band)
		}
	default:
		n := dsp.SamplesU8ToComplex(samples, r.iqBuf)
		if n < dsp.FFTSize {
			r.maybeEmitProtocol()
			return
		}
		iq := r.iqBuf[:n]
		if !r.protoScan.fullScan {
			r.appendDwellIQ(iq, band)
			r.emitProtocolMonitorAudio(iq, band)
		}
		r.emitProtocolSpectrum(iq, band)
		dsp.PowerSpectrum(iq, spectrumSegs, r.fftBuf, r.window, r.powBuf, r.specOut)
		r.protoScan.accumFrames++
		inv := float32(1.0 / float64(r.protoScan.accumFrames))
		for i, v := range r.specOut {
			if r.protoScan.accumFrames == 1 {
				r.protoScan.accum[i] = v
			} else {
				r.protoScan.accum[i] = r.protoScan.accum[i]*(1-inv) + v*inv
			}
		}
		if !r.protoScan.fullScan && protocol.NeedsCWAnalysis(band) {
			row := make([]float32, len(r.specOut))
			copy(row, r.specOut)
			r.protoScan.cwSpectra = append(r.protoScan.cwSpectra, row)
		}
	}

	if !r.protocolChunkReady(band, now) {
		r.maybeEmitProtocol()
		return
	}

	var chunkSignals []protocol.Signal
	if band.Decode == "" && r.protoScan.accumFrames > 0 {
		chunkSignals = r.finishSpectrumBand(band)
	}
	if band.Decode == "adsb" || band.Decode == "ais" {
		r.syncDecodedToProtocol()
		chunkSignals = protocol.SignalsForBand(r.trackerSnapshot(), band)
	}

	if r.protoScan.fullScan {
		r.recordFullScanBand(band, chunkSignals)
		if r.protoScan.bandIdx+1 >= len(bands) {
			r.completeFullScan()
			return
		}
	}

	r.advanceProtocolBand(now)
	r.maybeEmitProtocol()
}

func (r *Receiver) protocolChunkReady(band protocol.Band, now time.Time) bool {
	elapsed := now.Sub(r.protoScan.bandStart)
	if !r.protoScan.fullScan {
		return elapsed >= band.Dwell
	}
	if band.Decode == "adsb" || band.Decode == "ais" {
		return elapsed >= band.Dwell
	}
	if elapsed < protocol.FullScanTuneSettle {
		return false
	}
	return r.protoScan.accumFrames >= protocol.FullScanMinFFTFrames
}

func (r *Receiver) trackerSnapshot() []protocol.Signal {
	if !r.protoScan.fullScan && !r.protoScan.analyzing {
		return r.protoScan.tracker.Snapshot()
	}
	all := r.protoScan.tracker.SnapshotForFullScan()
	out := make([]protocol.Signal, 0, len(all))
	for _, s := range all {
		if protocol.FullScanKeepSignal(s) {
			out = append(out, s)
		}
	}
	return out
}

func (r *Receiver) finishSpectrumBand(band protocol.Band) []protocol.Signal {
	peaks := protocol.DetectPeaks(r.protoScan.accum, band.CenterHz, band.RateHz)
	peaks = protocol.SnapCellularPeaks(peaks, band)
	if protocol.IsWeatherAPTBand(band) {
		peaks = protocol.CollapseAPTPeaks(peaks)
	}
	iq := r.protoScan.dwellIQ
	sr := float64(band.RateHz)
	spectra := r.protoScan.cwSpectra
	var found []protocol.Signal

	for _, p := range peaks {
		peak := p
		if protocol.IsWeatherAPTBand(band) && protocol.IsNearAPTDownlink(p.FreqHz) {
			peak.FreqHz = protocol.SnapAPTDownlink(p.FreqHz)
		}
		sig := protocol.Classify(peak, band)
		if len(iq) >= 4096 {
			if r.protoScan.fullScan {
				if protocol.NeedsCWAnalysis(band) && protocol.IsCWPeak(peak) {
					if dr, ok := protocol.TryCWPeak(peak, band, iq, sr, band.CenterHz, spectra); ok {
						sig = protocol.MergeDecode(sig, dr)
					}
				}
			} else if dr, ok := protocol.TryDecodePeak(peak, band, iq, band.CenterHz, sr, nil); ok {
				sig = protocol.MergeDecode(sig, dr)
			}
		}
		if r.protoScan.fullScan && !protocol.FullScanKeepSignal(sig) {
			continue
		}
		r.protoScan.tracker.Upsert(sig)
		found = append(found, sig)
	}
	return found
}

func (r *Receiver) recordFullScanBand(band protocol.Band, chunkSignals []protocol.Signal) {
	cp := append([]protocol.Signal(nil), chunkSignals...)
	r.protoScan.bandRecords = append(r.protoScan.bandRecords, protocol.BandRecord{
		Band:    band,
		Signals: cp,
	})
}

func (r *Receiver) completeFullScan() {
	r.protoScan.analyzing = true
	bands := r.protoScan.bands()
	allSignals := protocol.AggregateSignalsFromRecords(r.protoScan.bandRecords)
	filtered := make([]protocol.Signal, 0, len(allSignals))
	for _, s := range allSignals {
		if protocol.FullScanKeepSignal(s) {
			filtered = append(filtered, s)
		}
	}
	allSignals = filtered
	total := len(bands)
	last := bands[total-1]
	progress := &protocol.ScanProgress{
		BandIdx:   total - 1,
		BandTotal: total,
		BandName:  last.Name,
		CenterHz:  last.CenterHz,
		RateHz:    last.RateHz,
		Pct:       100,
		Phase:     "analyzing",
	}
	r.pushDecode(DecodeFrame{
		Service:      ServiceProtocol,
		Signals:      allSignals,
		ScanBand:     last.Name,
		ScanProgress: progress,
	})

	progress = &protocol.ScanProgress{
		BandIdx:   total - 1,
		BandTotal: total,
		BandName:  last.Name,
		CenterHz:  last.CenterHz,
		RateHz:    last.RateHz,
		Pct:       100,
		Phase:     "done",
	}
	r.protoScan.listening = false
	r.protoScan.fullScan = false
	r.protoScan.analyzing = false
	r.protoScan.sweep = nil
	r.pushDecode(DecodeFrame{
		Service:          ServiceProtocol,
		Signals:          allSignals,
		ScanBand:         "",
		ScanProgress:     progress,
		FullScanComplete: true,
	})
}

func (r *Receiver) advanceProtocolBand(now time.Time) {
	bands := r.protoScan.bands()
	r.protoScan.bandIdx++
	if r.protoScan.bandIdx >= len(bands) {
		r.protoScan.bandIdx = 0
	}
	r.protoScan.accumFrames = 0
	r.protoScan.dwellIQ = r.protoScan.dwellIQ[:0]
	r.protoScan.cwSpectra = r.protoScan.cwSpectra[:0]
	r.protoScan.lastSpecEmit = time.Time{}
	for i := range r.protoScan.accum {
		r.protoScan.accum[i] = 0
	}
	r.tuneProtocolBand(r.protoScan.bandIdx)
}

func (r *Receiver) syncDecodedToProtocol() {
	for _, ac := range r.adsbTracker.Snapshot(3 * time.Minute) {
		r.protoScan.tracker.Upsert(protocol.FromAircraft(ac))
	}
	for _, v := range r.aisTracker.Snapshot(15 * time.Minute) {
		r.protoScan.tracker.Upsert(protocol.FromVessel(v))
	}
}

func (r *Receiver) scanProgressLocked() *protocol.ScanProgress {
	if !r.protoScan.listening && !r.protoScan.analyzing {
		return nil
	}
	total := len(r.protoScan.bands())
	if total == 0 {
		return nil
	}
	idx := r.protoScan.bandIdx
	if idx < 0 || idx >= total {
		idx = 0
	}
	band := r.protoScan.bands()[idx]
	dwellFrac := 0.0
	if band.Dwell > 0 {
		dwellFrac = time.Since(r.protoScan.bandStart).Seconds() / band.Dwell.Seconds()
		if dwellFrac > 1 {
			dwellFrac = 1
		}
	}
	phase := "scanning"
	if r.protoScan.analyzing {
		phase = "analyzing"
	}
	return &protocol.ScanProgress{
		BandIdx:   idx,
		BandTotal: total,
		BandName:  band.Name,
		CenterHz:  band.CenterHz,
		RateHz:    band.RateHz,
		Pct:       (float64(idx) + dwellFrac) / float64(total) * 100,
		Phase:     phase,
	}
}

func (r *Receiver) maybeEmitProtocol() {
	if !r.protoScan.listening && !r.protoScan.analyzing {
		return
	}
	now := time.Now()
	minInterval := protocolEmitMin
	if r.protoScan.fullScan {
		minInterval = fullScanEmitMin
	}
	if !r.protoScan.lastEmit.IsZero() && now.Sub(r.protoScan.lastEmit) < minInterval {
		return
	}
	r.protoScan.lastEmit = now
	r.syncDecodedToProtocol()

	bands := r.protoScan.bands()
	scanBand := ""
	if r.protoScan.bandIdx >= 0 && r.protoScan.bandIdx < len(bands) {
		scanBand = bands[r.protoScan.bandIdx].Name
	}
	frame := DecodeFrame{
		Service:  ServiceProtocol,
		Signals:  r.trackerSnapshot(),
		ScanBand: scanBand,
	}
	if r.protoScan.fullScan || r.protoScan.analyzing {
		frame.ScanProgress = r.scanProgressLocked()
	}
	r.pushDecode(frame)
}

func (r *Receiver) currentTuning() (center uint64, rate uint) {
	cfg := r.Config()
	switch cfg.Service {
	case ServiceProtocol:
		bands := r.protoScan.bands()
		if r.protoScan.bandIdx >= 0 && r.protoScan.bandIdx < len(bands) {
			b := bands[r.protoScan.bandIdx]
			return b.CenterHz, b.RateHz
		}
		if len(bands) > 0 {
			return bands[0].CenterHz, bands[0].RateHz
		}
		return cfg.CenterFreq, protocolSampleRate
	case ServiceAPT:
		return r.aptTuning()
	case ServiceLRPT:
		return r.lrptTuning()
	case ServiceMeteor:
		return r.meteorTuning()
	default:
		return effectiveTuning(cfg)
	}
}
