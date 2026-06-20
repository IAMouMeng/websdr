// Package rtlextra extends hz.tools/sdr/rtl with librtlsdr features not
// exposed by the upstream Go wrapper (direct sampling, offset tuning).
package rtlextra

/*
#cgo pkg-config: librtlsdr
#include <rtl-sdr.h>
*/
import "C"

import (
	"fmt"
	"reflect"
	"unsafe"

	"hz.tools/sdr/rtl"
)

// DirectSampling mode values for rtlsdr_set_direct_sampling.
const (
	DirectOff = 0
	DirectI   = 1
	DirectQ   = 2
)

func deviceHandle(dev *rtl.Sdr) *C.rtlsdr_dev_t {
	v := reflect.ValueOf(dev).Elem()
	f := v.FieldByName("handle")
	ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	return (*C.rtlsdr_dev_t)(ptr.UnsafePointer())
}

// SetDirectSampling enables or disables RTL2832 direct sampling (HF receive).
// mode: 0 off, 1 I-ADC, 2 Q-ADC.
func SetDirectSampling(dev *rtl.Sdr, mode int) error {
	if dev == nil {
		return fmt.Errorf("rtlextra: nil device")
	}
	if mode < DirectOff || mode > DirectQ {
		return fmt.Errorf("rtlextra: invalid direct sampling mode %d", mode)
	}
	h := deviceHandle(dev)
	if C.rtlsdr_set_direct_sampling(h, C.int(mode)) != 0 {
		return fmt.Errorf("rtlextra: set direct sampling %d", mode)
	}
	// Direct sampling conflicts with offset tuning; restore offset tuning for
	// normal R820T VHF/UHF (ADS-B, etc.) when leaving HF mode.
	if mode == DirectOff {
		C.rtlsdr_set_offset_tuning(h, 1) // best-effort; unsupported on some tuners
	} else {
		C.rtlsdr_set_offset_tuning(h, 0)
	}
	return nil
}
