package protocol

import (
	"fmt"
	"math"
	"strings"

	"github.com/iamoumeng/websdr/internal/dsp"
)

const rdsSymbolRate = 1187.5

var rdsOffsetWords = [4]uint16{0x0FC, 0x198, 0x348, 0x098}

var ptyNames = []string{
	"无", "新闻", "时事", "信息", "体育", "教育", "戏剧", "文化",
	"科学", "谈话", "流行音乐", "摇滚", "轻音乐", "古典", "其他音乐", "天气",
	"财经", "儿童", "社会", "宗教", "电话", "旅行", "休闲", "爵士",
	"乡村", "民族", "怀旧", "民谣", "文档", "测试", "警报", "紧急",
}

// RDSInfo holds decoded FM RDS metadata.
type RDSInfo struct {
	PI  string
	PS  string
	PTY string
}

// DecodeRDS extracts PI / PS / PTY from de-emphasized FM multiplex audio.
func DecodeRDS(audio []float32, sampleRate float64) (RDSInfo, bool) {
	if len(audio) < int(sampleRate*0.4) {
		return RDSInfo{}, false
	}

	iMix := mixRDS(audio, sampleRate, 57_000)
	var lpf dsp.FIR
	lpf.ProcessReal(iMix, 2400, sampleRate)

	symLen := sampleRate / rdsSymbolRate
	if symLen < 8 {
		return RDSInfo{}, false
	}

	bestBits, bestScore := tryRDSBits(iMix, symLen)
	if bestScore < 3 || len(bestBits) < 104 {
		return RDSInfo{}, false
	}

	info, ok := parseRDSGroups(bestBits)
	return info, ok
}

func mixRDS(audio []float32, sr, freq float64) []float32 {
	out := make([]float32, len(audio))
	phase := 0.0
	step := 2 * math.Pi * freq / sr
	for i, v := range audio {
		out[i] = v * float32(math.Cos(phase))
		phase += step
		if phase > 2*math.Pi {
			phase -= 2 * math.Pi
		}
	}
	return out
}

func tryRDSBits(iMix []float32, symLen float64) ([]byte, int) {
	phaseSamples := int(symLen)
	if phaseSamples < 1 {
		phaseSamples = 1
	}
	bestScore := 0
	var best []byte
	for phase := 0; phase < phaseSamples; phase++ {
		bits := diffDecodeRDS(iMix, symLen, phase)
		score := scoreRDSBlocks(bits)
		if score > bestScore {
			bestScore = score
			best = bits
		}
	}
	return best, bestScore
}

func diffDecodeRDS(iMix []float32, symLen float64, phase int) []byte {
	nSyms := int(float64(len(iMix)-phase) / symLen)
	if nSyms < 26 {
		return nil
	}
	bits := make([]byte, 0, nSyms)
	prev := 0
	for s := 0; s < nSyms; s++ {
		idx := phase + int((float64(s)+0.5)*symLen)
		if idx >= len(iMix) {
			break
		}
		sign := 0
		if iMix[idx] >= 0 {
			sign = 1
		}
		bit := sign ^ prev
		prev = sign
		bits = append(bits, byte(bit))
	}
	return bits
}

func scoreRDSBlocks(bits []byte) int {
	score := 0
	for i := 0; i+26 <= len(bits); i++ {
		if _, _, ok := decodeRDSBlock(bitsToU32(bits[i : i+26])); ok {
			score++
		}
	}
	return score
}

func parseRDSGroups(bits []byte) (RDSInfo, bool) {
	var (
		pi     uint16
		piOK   bool
		ps     [8]byte
		psSet  [8]bool
		pty    int
		ptyOK  bool
		groups int
	)

	for i := 0; i+104 <= len(bits); i++ {
		blockA, typA, okA := decodeRDSBlock(bitsToU32(bits[i : i+26]))
		blockB, typB, okB := decodeRDSBlock(bitsToU32(bits[i+26 : i+52]))
		blockC, typC, okC := decodeRDSBlock(bitsToU32(bits[i+52 : i+78]))
		blockD, typD, okD := decodeRDSBlock(bitsToU32(bits[i+78 : i+104]))
		if !okA || !okB || !okC || !okD || typA != 0 || typB != 1 || typC != 2 || typD != 3 {
			continue
		}
		groups++
		pi = blockA
		piOK = true

		gt := (blockB >> 11) & 0x1F
		if gt == 0 {
			if !ptyOK {
				pty = int((blockB >> 5) & 0x1F)
				ptyOK = true
			}
			offset := 0
			if blockB&0x10 != 0 {
				offset = 4
			}
			chars := []byte{
				byte(blockB & 0xFF),
				byte(blockC >> 8), byte(blockC & 0xFF),
				byte(blockD >> 8), byte(blockD & 0xFF),
			}
			for j, c := range chars {
				if c == 0 || c == 0xFF {
					continue
				}
				idx := offset + j
				if idx < 8 {
					ps[idx] = c
					psSet[idx] = true
				}
			}
		}
	}

	if !piOK || groups == 0 {
		return RDSInfo{}, false
	}

	info := RDSInfo{
		PI: fmt.Sprintf("0x%04X", pi),
	}
	if ptyOK && pty >= 0 && pty < len(ptyNames) {
		info.PTY = fmt.Sprintf("%s (%d)", ptyNames[pty], pty)
	}
	var psBuilder strings.Builder
	for i := 0; i < 8; i++ {
		if psSet[i] {
			psBuilder.WriteByte(ps[i])
		}
	}
	info.PS = strings.TrimSpace(psBuilder.String())
	if info.PS == "" {
		info.PS = "—"
	}
	return info, true
}

func decodeRDSBlock(raw uint32) (data uint16, blockType int, ok bool) {
	syn := rdsSyndrome(raw & 0x3FFFFFF)
	for i, off := range rdsOffsetWords {
		if syn == off {
			return uint16((raw >> 10) & 0xFFFF), i, true
		}
	}
	return 0, 0, false
}

func rdsSyndrome(block uint32) uint16 {
	reg := block & 0x3FFFFFF
	for i := 0; i < 16; i++ {
		if reg&0x2000000 != 0 {
			reg = (reg << 1) ^ 0x15D867A
		} else {
			reg <<= 1
		}
	}
	return uint16(reg>>16) & 0x3FF
}

func bitsToU32(bits []byte) uint32 {
	var v uint32
	for _, b := range bits {
		v = (v << 1) | uint32(b&1)
	}
	return v
}
