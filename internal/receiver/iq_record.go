package receiver

import (
	"hz.tools/sdr"
)

// IQFrame is a chunk of raw IQ samples for client-side recording.
type IQFrame struct {
	CenterHz uint64
	Rate     uint
	Channels uint8 // 1 = I only, 2 = interleaved IQ
	Data     []byte
}

type meteorRecordState struct {
	active   bool
	paused   bool
	channels int // 1 or 2
}

func (r *Receiver) initMeteorRecord() {
	r.meteorRecord = meteorRecordState{}
}

// SetMeteorRecord configures IQ capture for the meteor service.
func (r *Receiver) SetMeteorRecord(active, paused bool, channels int) {
	if channels != 1 && channels != 2 {
		channels = 2
	}
	r.enqueue(func() {
		r.meteorRecord.active = active
		r.meteorRecord.paused = paused
		r.meteorRecord.channels = channels
	})
}

func (r *Receiver) pushMeteorIQ(samples sdr.SamplesU8) {
	if !r.meteorRecord.active || r.meteorRecord.paused || len(samples) < 2 {
		return
	}
	center := r.meteorListen.centerFreqHz
	rate := r.meteorListen.sampleRateHz
	if center == 0 {
		center = meteorDefaultCenter
	}
	if rate == 0 {
		rate = meteorDefaultRate
	}

	rawLen := len(samples)
	var data []byte
	ch := uint8(2)
	if r.meteorRecord.channels == 1 {
		ch = 1
		data = make([]byte, rawLen)
		for i := 0; i < rawLen; i++ {
			data[i] = byte(int(samples[i][0]) - 128)
		}
	} else {
		data = make([]byte, rawLen*2)
		for i := 0; i < rawLen; i++ {
			data[i*2] = byte(int(samples[i][0]) - 128)
			data[i*2+1] = byte(int(samples[i][1]) - 128)
		}
	}

	frame := IQFrame{CenterHz: center, Rate: rate, Channels: ch, Data: data}
	select {
	case r.iqCh <- frame:
	default:
	}
}

func (r *Receiver) IQChan() <-chan IQFrame { return r.iqCh }
