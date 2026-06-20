// Package receiver drives the RTL-SDR device and turns its IQ stream into a
// spectrum feed and a demodulated 48 kHz audio feed.
package receiver

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"hz.tools/rf"
	"hz.tools/sdr"
	"hz.tools/sdr/rtl"

	"github.com/iamoumeng/websdr/internal/adsb"
	"github.com/iamoumeng/websdr/internal/ais"
	"github.com/iamoumeng/websdr/internal/dsp"
	"github.com/iamoumeng/websdr/internal/protocol"
)

// Service selects what the single tuner is doing. The RTL-SDR can only listen
// to one band at a time, so the active page picks a service and the receiver
// retunes the hardware and switches its processing accordingly.
type Service string

const (
	ServiceRadio    Service = "radio"
	ServiceADSB     Service = "adsb"
	ServiceAIS      Service = "ais"
	ServiceProtocol Service = "protocol"
	ServiceAPT      Service = "apt"
	ServiceLRPT     Service = "lrpt"
	ServiceMeteor   Service = "meteor"
)

// Fixed hardware tuning for the digital services.
const (
	adsbCenterFreq = 1090000000
	adsbSampleRate = 2000000
	aisCenterFreq  = 162000000
	aisSampleRate  = 1536000 // 32x the 48 kHz per-channel working rate
)

type Mode string

const (
	ModeFM  Mode = "fm"
	ModeAM  Mode = "am"
	ModeWFM Mode = "wfm"
	ModeUSB Mode = "usb"
	ModeLSB Mode = "lsb"
	ModeCW  Mode = "cw"
	ModeDSB Mode = "dsb"
	ModeRAW Mode = "raw"
)

// Spectrum dB values are quantized to a byte over this fixed range for the
// wire protocol. The frontend decodes with the same constants. The scale is
// uncalibrated (relative to full-scale IQ), which is fine for display.
const (
	specDBMin = -120.0
	specDBMax = 0.0
)

// spectrumFPS is the target spectrum/waterfall update rate. The IQ read block
// is sized from the sample rate to hit roughly this many frames per second.
const spectrumFPS = 30

// audioChunkSamples is the PCM frame size pushed to clients (~50 ms @ 48 kHz).
const audioChunkSamples = 2400

type Config struct {
	DeviceIndex uint
	SampleRate  uint
	CenterFreq  uint64
	TuneFreq    uint64
	Gain        float32
	AGC         bool
	Mode        Mode
	FilterBW    float64 // demod channel bandwidth, Hz
	CWPitch     float64 // CW beat note, Hz
	NR          bool    // spectral noise reduction on the audio
	NRLevel          float64 // noise-reduction aggressiveness, 0..1
	Service          Service // active receive service
	DirectSampling   int     // 0 off, 1 I-ADC, 2 Q-ADC (HF)
}

type SpectrumFrame struct {
	Data       []byte
	CenterFreq uint64
	TuneFreq   uint64
	SampleRate uint
	FilterBW   float64
}

type AudioFrame struct {
	PCM  []int16
	Rate int
}

// Receiver owns the SDR and all DSP scratch state. All hardware control and
// all DSP-state mutation happen on the single Start() goroutine; setters only
// touch config (under mu) and enqueue work via cmdCh, which keeps the RTL-SDR
// async reader free of cross-thread interference.
type Receiver struct {
	mu     sync.RWMutex
	config Config

	sdr       *rtl.Sdr
	running   atomic.Bool
	enabled   atomic.Bool
	parentCtx context.Context
	runCancel context.CancelFunc
	cmdCh     chan func()
	streamMu  sync.Mutex // serializes stream stop/start (SetEnabled, reopen)

	// demodulator state (read goroutine only)
	fmPrev   complex128
	mixPhase float64
	bfoPhase float64
	dcAcc    float32
	deAcc    float32
	resamp   dsp.Resampler
	agc      dsp.AGC
	aaFilt   dsp.FIR        // anti-alias low-pass applied before decimation
	chFilt   dsp.FIR        // channel selectivity filter (complex baseband)
	audFilt  dsp.FIR        // post-demod audio low-pass
	nr       dsp.SpectralNR // spectral noise reduction (48 kHz audio)

	// spectrum scratch
	window     []float64
	fftBuf     []complex128
	powBuf     []float64
	specOut    []float32
	specSmooth []float32

	// audio scratch
	iqBuf      []complex128
	workBuf    []complex128
	decimBuf   []complex128
	cwBuf      []complex128
	audioF     []float32
	pcmF       []float32
	audioI     []int16
	audioAccum []int16

	audioCh    chan AudioFrame
	spectrumCh chan SpectrumFrame
	iqCh       chan IQFrame

	// digital service decoders (read goroutine only)
	adsbDemod      *adsb.Demodulator
	adsbTracker    *adsb.Tracker
	aisChannels    []*aisChannel
	aisTracker     *ais.Tracker
	lastDecodeEmit time.Time
	decodeCh       chan DecodeFrame
	protoScan      protocolScanState
	aptListen      aptListenState
	lrptListen     lrptListenState
	meteorListen   meteorListenState
	meteorRecord   meteorRecordState
	meteorDemod    *protocol.MeteorDemod

	appliedDirectMode int // last HF direct-sampling mode applied to hardware (-1 = unknown)

	reopenReq          chan reopenRequest // async device reopen (must not run on read goroutine)
	reopenLoopStarted  atomic.Bool
	runGen             atomic.Uint64 // bumped on stop; stale drainPipe must not clear running
	rxMu               sync.Mutex
	rx                 sdr.ReadCloser // active IQ stream; closed in stopRun to unblock drainPipe
}

type reopenRequest struct {
	service Service
	same    bool
	after   func() // optional callback on the read goroutine after reopen
}

func New(cfg Config) (*Receiver, error) {
	applyDefaults(&cfg)

	r := &Receiver{
		config:            cfg,
		cmdCh:             make(chan func(), 32),
		appliedDirectMode: -1,
		window:     dsp.HannWindow(dsp.FFTSize),
		fftBuf:     make([]complex128, dsp.FFTSize),
		powBuf:     make([]float64, dsp.FFTSize/2),
		specOut:    make([]float32, dsp.FFTSize/2),
		specSmooth: make([]float32, dsp.FFTSize/2),
		iqBuf:      make([]complex128, maxBlockSamples),
		workBuf:    make([]complex128, maxBlockSamples),
		decimBuf:   make([]complex128, 16384),
		cwBuf:      make([]complex128, 16384),
		audioF:     make([]float32, 16384),
		pcmF:       make([]float32, 4096),
		audioI:     make([]int16, 4096),
		audioAccum: make([]int16, 0, audioChunkSamples*4),
		audioCh:    make(chan AudioFrame, 128),
		spectrumCh: make(chan SpectrumFrame, 4),
		iqCh:       make(chan IQFrame, 64),
		decodeCh:   make(chan DecodeFrame, 32),
		reopenReq:  make(chan reopenRequest, 32),
	}
	for i := range r.specSmooth {
		r.specSmooth[i] = specDBMin
	}
	r.initDecoders()
	r.initProtocolScan()
	r.initAPTListen()
	r.initLRPTListen()
	r.initMeteorListen()
	r.initMeteorRecord()
	if err := r.openDevice(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Receiver) openDevice() error {
	cfg := r.Config()
	dev, err := rtl.New(cfg.DeviceIndex, 16*32*512)
	if err != nil {
		return fmt.Errorf("open rtl-sdr: %w", err)
	}
	if info, err := rtl.InfoByDeviceIndex(cfg.DeviceIndex); err == nil {
		log.Printf("RTL-SDR: %s %s (SN: %s)", info.Manufacturer, info.Product, info.Serial)
	}
	// Tune for whatever service is active, not just the radio band — a reopen can
	// happen while a digital service is selected (e.g. resuming after disable, or
	// crossing the direct-sampling boundary into ADS-B).
	center, rate := effectiveTuning(cfg)
	if cfg.Service == ServiceProtocol {
		center, rate = r.protocolBandTuning()
		r.mu.Lock()
		r.config.CenterFreq = center
		r.config.SampleRate = rate
		r.mu.Unlock()
	}
	if err := dev.SetSampleRate(rate); err != nil {
		dev.Close()
		return err
	}
	if err := dev.SetCenterFrequency(rf.Hz(center)); err != nil {
		dev.Close()
		return err
	}
	if err := applyGain(dev, cfg); err != nil {
		dev.Close()
		return err
	}
	r.sdr = dev
	r.appliedDirectMode = openDeviceExtras(dev, cfg)
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 2048000
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeWFM
	}
	if cfg.FilterBW <= 0 {
		cfg.FilterBW = defaultFilterBW(cfg.Mode)
	}
	if cfg.CWPitch <= 0 {
		cfg.CWPitch = 700
	}
	if cfg.NRLevel <= 0 {
		cfg.NRLevel = 0.6
	}
	if cfg.TuneFreq == 0 {
		cfg.TuneFreq = cfg.CenterFreq
	}
	if cfg.CenterFreq == 0 {
		cfg.CenterFreq = cfg.TuneFreq
	}
	if cfg.Service == "" {
		cfg.Service = ServiceRadio
	}
	cfg.DirectSampling = directSamplingForFreq(cfg.TuneFreq)
	if directSamplingActive(*cfg) && cfg.SampleRate > 1_024_000 {
		cfg.SampleRate = 1_024_000
	}
}

func applyGain(dev *rtl.Sdr, cfg Config) error {
	if err := dev.SetAutomaticGain(cfg.AGC); err != nil {
		return err
	}
	if cfg.AGC {
		return nil
	}
	stages, err := dev.GetGainStages()
	if err != nil {
		return err
	}
	if len(stages) == 0 {
		return nil
	}
	return dev.SetGain(stages[0], cfg.Gain)
}

func (r *Receiver) SpectrumChan() <-chan SpectrumFrame { return r.spectrumCh }
func (r *Receiver) AudioChan() <-chan AudioFrame       { return r.audioCh }
func (r *Receiver) DecodeChan() <-chan DecodeFrame     { return r.decodeCh }

func (r *Receiver) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// enqueue schedules fn to run on the read goroutine. Before the receiver is
// started there is no consumer, so fn is run inline.
func (r *Receiver) enqueue(fn func()) {
	if !r.running.Load() {
		fn()
		return
	}
	r.cmdCh <- fn
}

// drainPendingCmds drops queued control callbacks. Called before stopping the
// read loop so stale tune/service commands cannot run against a reopened device.
func (r *Receiver) drainPendingCmds() {
	for {
		select {
		case <-r.cmdCh:
		default:
			return
		}
	}
}

// resetDSP clears all demodulator state. Must run on the read goroutine.
func (r *Receiver) resetDSP() {
	r.mixPhase = 0
	r.fmPrev = 0
	r.bfoPhase = 0
	r.dcAcc = 0
	r.deAcc = 0
	r.resamp.Reset()
	r.aaFilt.Reset()
	r.chFilt.Reset()
	r.audFilt.Reset()
	r.nr.Reset()
	r.audioAccum = r.audioAccum[:0]
	for i := range r.specSmooth {
		r.specSmooth[i] = specDBMin
	}
}

// maxTuneOffsetFrac bounds how far the tuned frequency may sit from the
// hardware center before we retune the hardware, keeping the demodulated
// channel away from the noisy band edges.
const maxTuneOffsetFrac = 0.45

// SetTuneFreq sets the demodulated frequency. While it stays within the
// current band it is purely a software mixing offset (no hardware change, so
// audio is uninterrupted). If it would fall outside the usable band, the
// hardware is recentered onto it.
func (r *Receiver) SetTuneFreq(freq uint64) {
	r.mu.Lock()
	if r.config.Service != ServiceRadio {
		r.mu.Unlock()
		return
	}
	r.config.TuneFreq = freq
	modeChanged := r.syncDirectSamplingLocked(freq)
	limit := float64(r.config.SampleRate) * maxTuneOffsetFrac
	offset := float64(freq) - float64(r.config.CenterFreq)
	recenter := offset > limit || offset < -limit
	if recenter {
		r.config.CenterFreq = freq
	}
	center := r.config.CenterFreq
	r.mu.Unlock()
	if modeChanged {
		r.reopenForReconfig(ServiceRadio, true)
		return
	}
	if recenter {
		r.tuneHardware(center, 0)
	}
}

// SetCenterFreq retunes the hardware center, leaving the tuned frequency where
// it is (it becomes an offset within the new band).
func (r *Receiver) SetCenterFreq(freq uint64) {
	r.mu.Lock()
	if r.config.Service != ServiceRadio {
		r.mu.Unlock()
		return
	}
	modeChanged := r.syncDirectSamplingLocked(r.config.TuneFreq)
	if r.config.CenterFreq == freq && !modeChanged {
		r.mu.Unlock()
		return
	}
	r.config.CenterFreq = freq
	center := r.config.CenterFreq
	r.mu.Unlock()
	if modeChanged {
		r.reopenForReconfig(ServiceRadio, true)
		return
	}
	r.tuneHardware(center, 0)
}

// tuneHardware changes the SDR center frequency on the read goroutine. It does
// NOT reset the USB buffer, which would stall the async stream and drop audio.
func (r *Receiver) tuneHardware(center uint64, rate uint) {
	r.enqueue(func() {
		if r.sdr == nil || r.Config().Service != ServiceRadio {
			return
		}
		cfg := r.Config()
		center = snapFreqForMode(center, cfg)
		r.applyRadioTuning(center, rate)
	})
}

func (r *Receiver) SetSampleRate(rate uint) {
	r.mu.Lock()
	if rate == r.config.SampleRate {
		r.mu.Unlock()
		return
	}
	r.config.SampleRate = rate
	r.mu.Unlock()
	r.enqueue(func() {
		if r.sdr != nil && r.Config().Service == ServiceRadio {
			if err := r.sdr.SetSampleRate(rate); err != nil {
				log.Printf("set sample rate: %v", err)
			}
		}
		r.resetDSP()
	})
}

func (r *Receiver) SetFilterBW(bw float64) {
	if bw <= 0 {
		return
	}
	r.mu.Lock()
	r.config.FilterBW = bw
	r.mu.Unlock()
}

func (r *Receiver) SetCWPitch(pitch float64) {
	if pitch <= 0 {
		return
	}
	r.mu.Lock()
	r.config.CWPitch = pitch
	r.mu.Unlock()
}

func (r *Receiver) SetNR(enabled bool) {
	r.mu.Lock()
	r.config.NR = enabled
	r.mu.Unlock()
	// Clear stale overlap-add state so toggling on doesn't replay old audio.
	r.enqueue(r.nr.Reset)
}

func (r *Receiver) SetNRLevel(level float64) {
	if level < 0 {
		level = 0
	} else if level > 1 {
		level = 1
	}
	r.mu.Lock()
	r.config.NRLevel = level
	r.mu.Unlock()
}

func (r *Receiver) SetGain(gain float32) {
	r.mu.Lock()
	r.config.Gain = gain
	r.config.AGC = false
	cfg := r.config
	r.mu.Unlock()
	r.enqueue(func() {
		if r.sdr != nil {
			if err := applyGain(r.sdr, cfg); err != nil {
				log.Printf("set gain: %v", err)
			}
		}
	})
}

func (r *Receiver) SetAGC(enabled bool) {
	r.mu.Lock()
	r.config.AGC = enabled
	cfg := r.config
	r.mu.Unlock()
	r.enqueue(func() {
		if r.sdr != nil {
			if err := applyGain(r.sdr, cfg); err != nil {
				log.Printf("set agc: %v", err)
			}
		}
	})
}

func (r *Receiver) SetMode(mode Mode) {
	r.mu.Lock()
	r.config.Mode = mode
	r.config.FilterBW = defaultFilterBW(mode)
	r.mu.Unlock()
	r.enqueue(r.resetDSP)
}

func (r *Receiver) Enabled() bool { return r.enabled.Load() }

func (r *Receiver) Start(ctx context.Context) error {
	r.parentCtx = ctx
	r.enabled.Store(true)
	if r.reopenLoopStarted.CompareAndSwap(false, true) {
		go r.reopenLoop()
	}
	return r.startRun()
}

func (r *Receiver) SetEnabled(on bool) error {
	r.streamMu.Lock()
	defer r.streamMu.Unlock()
	if on {
		r.enabled.Store(true)
		return r.startRun()
	}
	r.enabled.Store(false)
	return r.stopRun()
}

func (r *Receiver) startRun() error {
	if r.parentCtx == nil {
		return fmt.Errorf("receiver not initialized")
	}
	if r.sdr == nil {
		if err := r.openDevice(); err != nil {
			return err
		}
	}
	if !r.running.CompareAndSwap(false, true) {
		return fmt.Errorf("stream already running")
	}

	gen := r.runGen.Load()
	rx, err := r.sdr.StartRx()
	if err != nil {
		r.running.Store(false)
		return fmt.Errorf("start rx: %w", err)
	}
	r.rxMu.Lock()
	r.rx = rx
	r.rxMu.Unlock()

	runCtx, cancel := context.WithCancel(r.parentCtx)
	r.runCancel = cancel

	blockCh := make(chan sdr.SamplesU8, 8)
	go r.drainPipe(runCtx, gen, blockCh)
	go r.processLoop(runCtx, blockCh)
	return nil
}

// reopenLoop runs device reopens off the IQ read goroutine. It uses parentCtx
// so a stopRun during reopen does not kill this loop.
func (r *Receiver) reopenLoop() {
	for {
		select {
		case <-r.parentCtx.Done():
			return
		case first := <-r.reopenReq:
			req := first
			r.reopenForReconfig(req.service, req.same)
			if req.after != nil {
				r.enqueue(req.after)
			}
		}
	}
}

func (r *Receiver) pushReopenReq(req reopenRequest) {
	select {
	case r.reopenReq <- req:
	default:
		select {
		case dropped := <-r.reopenReq:
			if dropped.after != nil {
				r.enqueue(func() { r.protoScan.retunePending = false })
			}
		default:
		}
		r.reopenReq <- req
	}
}

func (r *Receiver) requestReopen(s Service, same bool, after func()) {
	r.pushReopenReq(reopenRequest{service: s, same: same, after: after})
}

func (r *Receiver) stopStreaming() error {
	// Invalidate any in-flight drainPipe so its defer cannot clear running
	// after a new startRun has already begun (caused nil deref in librtlsdr).
	r.runGen.Add(1)

	if r.runCancel != nil {
		r.runCancel()
		r.runCancel = nil
	}
	r.rxMu.Lock()
	if r.rx != nil {
		_ = r.rx.Close()
		r.rx = nil
	}
	r.rxMu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for r.running.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	r.running.Store(false)
	return nil
}

func (r *Receiver) stopRun() error {
	if err := r.stopStreaming(); err != nil {
		return err
	}
	if r.sdr != nil {
		if err := r.sdr.Close(); err != nil {
			return err
		}
		r.sdr = nil
	}
	return nil
}

// drainPipe does nothing but pull IQ blocks off the SDR pipe as fast as it can
// and hand them to processLoop. This is deliberately the ONLY thing it does.
//
// The rtl driver's C read callback writes into an unbuffered pipe and blocks
// until someone reads it; that callback runs on libusb's event thread, so
// while it is blocked libusb cannot reap or resubmit USB transfers and the
// dongle's FIFO overflows (after which it stops streaming until a buffer
// reset). Therefore this loop must never stall: it never executes device
// control, and if processLoop falls behind it DROPS the block rather than
// block the callback. Device control runs concurrently on processLoop, which
// is the librtlsdr-supported pattern (cf. rtl_tcp's command thread).
func (r *Receiver) drainPipe(ctx context.Context, gen uint64, blockCh chan<- sdr.SamplesU8) {
	defer func() {
		close(blockCh)
		if r.runGen.Load() == gen {
			r.running.Store(false)
		}
	}()

	_, curRate := r.currentTuning()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		r.rxMu.Lock()
		rx := r.rx
		r.rxMu.Unlock()
		if rx == nil {
			return
		}

		rawBuf := make(sdr.SamplesU8, blockSamples(curRate))
		n, err := rx.Read(rawBuf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("read samples: %v", err)
			continue
		}
		if n == 0 {
			continue
		}

		select {
		case blockCh <- rawBuf[:n]:
		case <-ctx.Done():
			return
		default:
			// processLoop is busy (typically applying a control command).
			// Drop this block to keep the USB stream alive — a brief audio
			// glitch is far better than a dead device.
		}

		if _, rate := r.currentTuning(); rate != curRate {
			curRate = rate
		}
	}
}

// processLoop applies pending device-control commands and runs DSP on the IQ
// blocks delivered by drainPipe. Running control here (not on the drain path)
// is what keeps a slow control transfer from stalling the USB callback.
func (r *Receiver) processLoop(ctx context.Context, blockCh <-chan sdr.SamplesU8) {
	for {
		select {
		case <-ctx.Done():
			return
		case block, ok := <-blockCh:
			if !ok {
				return
			}
			r.drainCommands()
			r.processBlock(block)
		}
	}
}

func (r *Receiver) drainCommands() {
	for {
		select {
		case fn := <-r.cmdCh:
			fn()
		default:
			return
		}
	}
}

func (r *Receiver) Close() error {
	r.enabled.Store(false)
	return r.stopRun()
}

func DeviceCount() uint { return rtl.DeviceCount() }

func defaultFilterBW(mode Mode) float64 {
	switch mode {
	case ModeCW:
		return 500
	case ModeUSB, ModeLSB:
		return 2400
	case ModeDSB:
		return 6000
	case ModeRAW:
		return 12500
	case ModeAM:
		return 6000
	case ModeFM:
		return 12500
	case ModeWFM:
		return 150000
	default:
		return 10000
	}
}

func DefaultConfig() Config {
	center := uint64(100000000)
	return Config{
		DeviceIndex: 0,
		SampleRate:  2048000,
		CenterFreq:  center,
		TuneFreq:    center,
		Gain:        20,
		AGC:         false,
		Mode:        ModeWFM,
		FilterBW:    150000,
		CWPitch:     700,
		NR:          false,
		NRLevel:     0.6,
	}
}
