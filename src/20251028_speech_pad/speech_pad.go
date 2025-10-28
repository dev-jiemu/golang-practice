package main

import (
	"math"
	"sort"
)

type PadMetrics struct {
	SNRdB             float64
	AvgSilenceSec     float64
	ShortSilenceRatio float64
	NoiseFloorRMS     float64
	SpeechLevelRMS    float64
	RawPadMs          float64
	FinalPadMs        int

	SuggestedMinSilenceMs int
}

func EstimateFileLevelPadAndMinSilence(pcm []float32, sr int) PadMetrics {
	m := PadMetrics{FinalPadMs: 160, SuggestedMinSilenceMs: 500}

	rms := frameRMS(pcm, sr, 0.020, 0.010)
	if len(rms) == 0 {
		return m
	}

	m.NoiseFloorRMS = percentile(rms, 20)
	m.SpeechLevelRMS = percentile(rms, 80)
	noise := m.NoiseFloorRMS
	if noise <= 1e-9 {
		noise = 1e-9
	}
	m.SNRdB = 20.0 * math.Log10(m.SpeechLevelRMS/noise)

	// 침묵 통계 (임계치 = noise*1.5)
	silThr := m.NoiseFloorRMS * 1.5
	m.AvgSilenceSec, m.ShortSilenceRatio = estimateSilenceStats(rms, 0.010, silThr)

	// ---------- Pad: SNR 좋으면 상한 낮춤 ----------
	var capHi float64
	switch {
	case m.SNRdB >= 18:
		capHi = 220
	case m.SNRdB >= 15:
		capHi = 260
	case m.SNRdB >= 10:
		capHi = 320
	default:
		capHi = 400
	}
	base := 150.0
	snrAdj := clamp((15.0-m.SNRdB)*8.0, 0, 160)
	silAdj := clamp((0.30-m.AvgSilenceSec)*300.0, 0, 180) // 평균 침묵이 0.3s보다 짧으면 증가
	shortAdj := clamp(m.ShortSilenceRatio*120.0, 0, 120)
	m.RawPadMs = base + snrAdj + silAdj + shortAdj
	m.FinalPadMs = int(clamp(m.RawPadMs, 100, capHi))

	// ---------- MinSilence 추천치 ----------
	// 기본값 500ms에서 시작:
	// - 짧은 침묵이 많을수록 ↑ (과분리 방지)
	// - SNR 낮을수록 ↑ (잡음으로 인한 깜빡임 방지)
	// - 평균 침묵이 충분히 길면 ↓ (과병합 방지)
	minSil := 500.0
	minSil += clamp(m.ShortSilenceRatio*400.0, 0, 300)    // 0~300ms 가산
	minSil += clamp((12.0-m.SNRdB)*10.0, 0, 200)          // SNR<12dB면 최대 +200ms
	minSil -= clamp((m.AvgSilenceSec-0.45)*500.0, 0, 200) // 평균 침묵>0.45s면 최대 -200ms

	// 하한/상한 (너무 과하게 변동하지 않게)
	m.SuggestedMinSilenceMs = int(clamp(minSil, 350, 750))
	return m
}

func frameRMS(pcm []float32, sr int, winSec, hopSec float64) []float64 {
	win := int(winSec * float64(sr))
	hop := int(hopSec * float64(sr))
	if win <= 0 || hop <= 0 || len(pcm) < win {
		return nil
	}
	out := make([]float64, 0, 1+len(pcm)/hop)
	for i := 0; i+win <= len(pcm); i += hop {
		var s float64
		for j := 0; j < win; j++ {
			x := float64(pcm[i+j])
			s += x * x
		}
		out = append(out, math.Sqrt(s/float64(win)))
	}
	return out
}

func estimateSilenceStats(rms []float64, hopSec float64, thr float64) (avgSilenceSec float64, shortRatio float64) {
	var cur float64
	var sils []float64
	for _, v := range rms {
		if v < thr {
			cur += hopSec
		} else if cur > 0 {
			sils = append(sils, cur)
			cur = 0
		}
	}
	if cur > 0 {
		sils = append(sils, cur)
	}
	if len(sils) == 0 {
		return 0, 0
	}
	var sum float64
	var short int
	for _, s := range sils {
		sum += s
		if s < 0.3 {
			short++
		} // 300ms 미만
	}
	return sum / float64(len(sils)), float64(short) / float64(len(sils))
}

func percentile(xs []float64, p int) float64 {
	if len(xs) == 0 {
		return 0
	}
	tmp := append([]float64(nil), xs...)
	sort.Float64s(tmp)
	if p <= 0 {
		return tmp[0]
	}
	if p >= 100 {
		return tmp[len(tmp)-1]
	}
	idx := int(math.Round((float64(p) / 100.0) * float64(len(tmp)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(tmp) {
		idx = len(tmp) - 1
	}
	return tmp[idx]
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
