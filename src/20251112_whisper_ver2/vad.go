package main

/*
// macOS용 (로컬 개발)
#cgo darwin CFLAGS: -I/usr/local/onnxruntime-osx-arm64-1.18.1/include
#cgo darwin LDFLAGS: -L/usr/local/onnxruntime-osx-arm64-1.18.1/lib -lonnxruntime -Wl,-rpath,/usr/local/onnxruntime-osx-arm64-1.18.1/lib
*/
import "C"
import (
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/labstack/gommon/log"
	"github.com/streamer45/silero-vad-go/speech"
)

type out0 []speech.Segment

const (
	softGateAtt = 0.1 // Whisper 환각 방지를 위해 완전히 0이 아닌 작은 값으로 감쇠
)

// VadFilter : 무음구간 처리 필터 호출
func VadFilter(config *speech.DetectorConfig, job *Job) ([]speech.Segment, *VADSidecar, error) {
	var err error

	var inputFile *os.File
	inputFile, err = os.Open(job.WavAudioPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open audio file: %v", err)
	}
	defer inputFile.Close()

	var decoder *wav.Decoder
	decoder = wav.NewDecoder(inputFile)
	if !decoder.IsValidFile() {
		return nil, nil, fmt.Errorf("audio file is not a valid file")
	}

	var buf *audio.IntBuffer
	buf, err = decoder.FullPCMBuffer()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read PCM buffer: %v", err)
	}

	pcmBuf := buf.AsFloat32Buffer()
	sampleRate := buf.Format.SampleRate
	if buf.Format.NumChannels != 1 { // 모노채널 체크 : ffmpeg으로 모노채널로 만들긴 하지만 혹시모를 방어코드임
		return nil, nil, fmt.Errorf("expected mono(1ch), got %dch (preprocess with -ac 1)", buf.Format.NumChannels)
	}

	if config == nil {
		return nil, nil, fmt.Errorf("speech config is nil")
	}

	// samplerate 16000 고정 : ffmpeg 으로 16000 지정하긴 하지만 혹시모를 방어코드임 22
	if sampleRate != 16000 {
		return nil, nil, fmt.Errorf("VAD requires 16kHz. got %d (resample with ffmpeg -ar 16000)", sampleRate)
	}
	config.SampleRate = 16000

	// 둘다 0일경우 지정해야 하는거고, 아닐 경우 테스트용으로 지정해서 설정한것으로 간주하고 해당 값 그대로 사용
	if config.SpeechPadMs == 0 && config.MinSilenceDurationMs == 0 {
		matrics := estimateFileLevelPadAndMinSilence(pcmBuf.Data, sampleRate)
		// SNRdB 10 이하, AvgSilenceSec 1.0 이상이 필터?
		log.Debug("Audio quality metrics", "snr_db", matrics.SNRdB, "avg_silence_sec", matrics.AvgSilenceSec)
		config.SpeechPadMs = matrics.FinalPadMs
		config.MinSilenceDurationMs = matrics.SuggestedMinSilenceMs
	}

	log.Debug("vad filter results", "speech_pad_ms", config.SpeechPadMs, "min_silence_duration_ms", config.MinSilenceDurationMs, "sample_rate", config.SampleRate)

	var sd *speech.Detector
	sd, err = speech.NewDetector(*config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create speech detector: %v", err)
	}
	defer sd.Destroy()

	var segments []speech.Segment
	segments, err = sd.Detect(pcmBuf.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to detect PCM segments: %v", err)
	}

	// ===== 디버깅 로그 추가 시작 =====
	log.Info("========================================")
	log.Infof("=== VAD Filter Debug: Step by Step ===")
	log.Info("========================================")

	rawSegmentCount := len(segments)
	log.Infof("Step 0: Raw VAD Detection")
	log.Infof("  - Segments: %d", rawSegmentCount)
	log.Infof("  - Threshold: %.2f", config.Threshold)

	if len(segments) > 0 {
		log.Infof("  - First segment: %.3fs - %.3fs (%.2fs)",
			segments[0].SpeechStartAt,
			segments[0].SpeechEndAt,
			segments[0].SpeechEndAt-segments[0].SpeechStartAt)
		log.Infof("  - Last segment: %.3fs - %.3fs (%.2fs)",
			segments[len(segments)-1].SpeechStartAt,
			segments[len(segments)-1].SpeechEndAt,
			segments[len(segments)-1].SpeechEndAt-segments[len(segments)-1].SpeechStartAt)
	}
	log.Info("----------------------------------------")

	rawSegments := make([]speech.Segment, len(segments))
	copy(rawSegments, segments)

	// 너무 짧은 세그먼트 제거 (1초 이하)
	filteredSegments := make([]speech.Segment, 0)
	removedByDuration := 0

	for _, seg := range segments {
		// end가 0이면 파일 끝까지라는 의미 -> duration 재계산
		endTime := seg.SpeechEndAt
		if endTime <= 0 {
			// 전체 오디오 길이로 대체
			endTime = float64(len(pcmBuf.Data)) / float64(sampleRate)
		}

		duration := endTime - seg.SpeechStartAt

		// duration이 여전히 이상하면 스킵
		if duration < 0 {
			log.Debug("비정상 세그먼트 스킵", "start", seg.SpeechStartAt, "end", seg.SpeechEndAt, "duration", duration)
			removedByDuration++
			continue
		}

		if duration >= 0.2 {
			// end를 보정한 세그먼트 추가
			correctedSeg := seg
			correctedSeg.SpeechEndAt = endTime
			filteredSegments = append(filteredSegments, correctedSeg)
		} else {
			removedByDuration++
		}
	}

	log.Infof("Step 1: Duration Filter (>= 0.2s)")
	log.Infof("  - Remaining: %d (removed: %d)", len(filteredSegments), removedByDuration)
	log.Info("----------------------------------------")
	segments = filteredSegments

	beforeMerge := len(segments)
	segments = mergeCloseSegments(segments, 1.5)
	log.Infof("Step 2: Merge Close Segments (<= 1.5s gap)")
	log.Infof("  - Before: %d, After: %d (merged: %d)", beforeMerge, len(segments), beforeMerge-len(segments))
	log.Infof("  - Average duration: %.2fs", calculateAvgDuration(segments))
	log.Info("----------------------------------------")

	// 히스테리시스 효과 : 짧은 무성 병합
	beforeShortGap := len(segments)
	segments = mergeShortGaps(segments, 600)
	log.Infof("Step 3: Merge Short Gaps (<= 600ms)")
	log.Infof("  - Before: %d, After: %d (merged: %d)", beforeShortGap, len(segments), beforeShortGap-len(segments))
	log.Info("----------------------------------------")

	// 진짜 짧은거 자르기
	beforeDropShort := len(segments)
	segments = dropShortSpeech(segments, 120)
	log.Infof("Step 4: Drop Short Speech (< 120ms)")
	log.Infof("  - Before: %d, After: %d (removed: %d)", beforeDropShort, len(segments), beforeDropShort-len(segments))
	log.Info("----------------------------------------")

	// 비대칭 패딩
	beforePadding := len(segments)
	segments = applyAsymmetricPaddingToSegments(segments, sampleRate, 300, 500)
	log.Infof("Step 5: Apply Padding (pre: 300ms, post: 500ms)")
	log.Infof("  - Before: %d, After: %d", beforePadding, len(segments))
	log.Infof("  - Average duration: %.2fs", calculateAvgDuration(segments))
	log.Info("----------------------------------------")

	// 특정 구간 (42~50초) 상세 분석
	// TODO : 특정 발화구간이 사라져서 데이터 체크용임
	log.Info("=== Analyzing 42s-50s region ===")
	targetStart := 42.0
	targetEnd := 50.0

	log.Infof("Raw VAD segments in region:")
	foundInRaw := false
	for i, seg := range rawSegments { // ← 이게 sd.Detect() 직후의 원본
		if seg.SpeechEndAt >= 40.0 && seg.SpeechStartAt <= 52.0 {
			inTarget := ""
			if seg.SpeechEndAt >= targetStart && seg.SpeechStartAt <= targetEnd {
				inTarget = " ← TARGET REGION"
			}
			log.Infof("  [Raw #%d] %.3fs - %.3fs (%.2fs)%s",
				i, seg.SpeechStartAt, seg.SpeechEndAt,
				seg.SpeechEndAt-seg.SpeechStartAt, inTarget)
		}
	}
	if !foundInRaw {
		log.Warn("  ⚠️  NO segments detected by VAD in this region!")
	}

	log.Infof("Final segments in region:")
	foundInFinal := false
	for i, seg := range segments {
		if seg.SpeechEndAt >= targetStart && seg.SpeechStartAt <= targetEnd {
			log.Infof("  [Final #%d] %.3fs - %.3fs (%.2fs)",
				i, seg.SpeechStartAt, seg.SpeechEndAt, seg.SpeechEndAt-seg.SpeechStartAt)
			foundInFinal = true
		}
	}
	if !foundInFinal {
		log.Warn("  ⚠️  NO segments survived filtering in this region!")
	}
	log.Info("========================================")

	log.Debug("post smooth/pad/chunk",
		"segments", len(segments),
		"avg_duration", fmt.Sprintf("%.2f", calculateAvgDuration(segments)),
	)

	// 원본 오디오 데이터를 복사 (전체 길이 유지)
	processedAudio := make([]float32, len(pcmBuf.Data))
	copy(processedAudio, pcmBuf.Data)

	// 디버깅: 원본 데이터의 샘플 값 확인
	var originalSum float64
	for _, v := range pcmBuf.Data {
		originalSum += float64(v * v)
	}
	originalRMS := math.Sqrt(originalSum / float64(len(pcmBuf.Data)))
	log.Debug("Before VAD filter", "original_rms", fmt.Sprintf("%.6f", originalRMS))

	rms20ms10ms := frameRMS(pcmBuf.Data, sampleRate, 0.020, 0.010)
	noise := percentile(rms20ms10ms, 20) // 노이즈 바닥
	energyThr := noise * 1.6             // 1.4~1.8 사이 튜닝 권장

	var silenced int
	if len(segments) == 0 {
		// VAD가 전부 놓쳤다면: 에너지 마스크로라도 구간을 살림
		energyMask := buildEnergyMask(pcmBuf.Data, sampleRate, 0.020, 0.010, energyThr)

		// 마스크가 거의 전부 false일 수도 있으니, 최소 완충을 위해 dilation
		dilateSpeechMask(energyMask, sampleRate, 150, 250)

		// 무음 구간을 softGateAtt 레벨로 감쇠
		for i := range processedAudio {
			if !energyMask[i] {
				processedAudio[i] *= softGateAtt
				silenced++
			}
		}

		// 경계 페이드 + 감쇠
		applyBoundaryFades(processedAudio, energyMask, sampleRate, 35, 45, softGateAtt)
	} else {
		speechRegions := make([]bool, len(pcmBuf.Data))

		// VAD 세그먼트로 마스크 생성
		for _, segment := range segments {
			startSample := int(segment.SpeechStartAt * float64(sampleRate))
			endSample := int(segment.SpeechEndAt * float64(sampleRate))
			if endSample <= 0 || endSample > len(pcmBuf.Data) {
				endSample = len(pcmBuf.Data)
			}
			if startSample < 0 {
				startSample = 0
			}
			if startSample >= endSample {
				continue
			}
			for j := startSample; j < endSample; j++ {
				speechRegions[j] = true
			}
		}

		// 에너지 마스크 생성 및 OR 결합
		energyMask := buildEnergyMask(pcmBuf.Data, sampleRate, 0.020, 0.010, energyThr)
		for i := range speechRegions {
			speechRegions[i] = speechRegions[i] || energyMask[i]
		}

		// 확장 및 페이드
		dilateSpeechMask(speechRegions, sampleRate, 200, 300)

		// 무음 구간을 softGateAtt 레벨로 감쇠
		for i := range processedAudio {
			if !speechRegions[i] {
				processedAudio[i] *= softGateAtt
				silenced++
			}
		}

		applyBoundaryFades(processedAudio, speechRegions, sampleRate, 35, 45, softGateAtt)
	}

	// 디버깅: 필터 적용 후 데이터 확인
	var processedSum float64
	var zeroCount int
	for _, v := range processedAudio {
		processedSum += float64(v * v)
		if v == 0 {
			zeroCount++
		}
	}
	processedRMS := math.Sqrt(processedSum / float64(len(processedAudio)))
	log.Debug("After VAD filter",
		"processed_rms", fmt.Sprintf("%.6f", processedRMS),
		"zero_samples", zeroCount,
		"zero_ratio", fmt.Sprintf("%.2f%%", float64(zeroCount)/float64(len(processedAudio))*100),
	)

	var outputFile *os.File
	outputFile, err = os.Create(job.FilteredAudioPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	var encoder *wav.Encoder
	encoder = wav.NewEncoder(outputFile, sampleRate, 16, buf.Format.NumChannels, 1)

	// Float32를 Int로 변환
	intData := make([]int, len(processedAudio))
	for i, sample := range processedAudio {
		if sample > 1 {
			sample = 1
		}
		if sample < -1 {
			sample = -1
		}
		intData[i] = int(sample * 32767.0)
	}

	var outputBuf *audio.IntBuffer
	outputBuf = &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: buf.Format.NumChannels,
			SampleRate:  sampleRate,
		},
		Data:           intData,
		SourceBitDepth: 16,
	}

	err = encoder.Write(outputBuf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write audio: %v", err)
	}

	err = encoder.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to close encoder: %v", err)
	}

	rms := frameRMS(pcmBuf.Data, sampleRate, 0.020, 0.010)
	m := estimateFileLevelPadAndMinSilence(pcmBuf.Data, sampleRate)

	sidecar := &VADSidecar{
		SampleRate:    sampleRate,
		HopSec:        0.010,
		NoiseFloorRMS: m.NoiseFloorRMS,
		RMSEnvelope:   rms,
	}

	return segments, sidecar, nil
}

// applyBoundaryFades : 경계에서 페이드인(non->speech)
func applyBoundaryFades(data []float32, speechMask []bool, sampleRate, fadeOutMs, fadeInMs int, softGate float32) {
	n := len(data)
	if n == 0 {
		return
	}
	fo := int(float64(sampleRate) * float64(fadeOutMs) / 1000.0)
	fi := int(float64(sampleRate) * float64(fadeInMs) / 1000.0)
	if fo < 1 {
		fo = 1
	}
	if fi < 1 {
		fi = 1
	}

	// speech -> non-speech : 페이드아웃 (코사인 곡선)
	for i := 1; i < n; i++ {
		if speechMask[i-1] && !speechMask[i] {
			end := i - 1
			start := end - fo + 1
			if start < 0 {
				start = 0
			}
			span := end - start + 1
			for k := 0; k < span; k++ {
				theta := float64(k+1) / float64(span+1) * math.Pi * 0.5
				alpha := float32(math.Cos(theta)) // 1..0
				data[start+k] *= alpha
			}
		}
	}

	// non-speech -> speech : 페이드인 (softGate -> 1.0)
	for i := 1; i < n; i++ {
		if !speechMask[i-1] && speechMask[i] {
			start := i
			end := start + fi - 1
			if end >= n {
				end = n - 1
			}
			span := end - start + 1
			for k := 0; k < span; k++ {
				theta := float64(k+1) / float64(span+1) * math.Pi * 0.5
				alpha := float32(math.Sin(theta)) // 0..1
				gain := softGate + (1.0-softGate)*alpha
				data[start+k] *= gain
			}
		}
	}
}

// dilateSpeechMask : speechMask를 앞/뒤로 확장해 경계 누락 메움
func dilateSpeechMask(mask []bool, sr int, preMs, postMs int) {
	n := len(mask)
	pre := int(float64(sr) * float64(preMs) / 1000.0)
	post := int(float64(sr) * float64(postMs) / 1000.0)
	if pre < 0 {
		pre = 0
	}
	if post < 0 {
		post = 0
	}

	src := make([]bool, n)
	copy(src, mask)

	for i := 0; i < n; i++ {
		if src[i] {
			st := i - pre
			if st < 0 {
				st = 0
			}
			en := i + post
			if en >= n {
				en = n - 1
			}
			for j := st; j <= en; j++ {
				mask[j] = true
			}
		}
	}
}

// estimateFileLevelPadAndMinSilence : prc, sr 기준으로 speech_pad, min_silence_duration_ms 설정(파일에 따른 유동화)
func estimateFileLevelPadAndMinSilence(pcm []float32, sr int) PadMetrics {
	m := PadMetrics{FinalPadMs: 200, SuggestedMinSilenceMs: 800}

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

	// 침묵 통계 (임계치 = noise*2.0)
	silThr := m.NoiseFloorRMS * 2.0
	m.AvgSilenceSec, m.ShortSilenceRatio = estimateSilenceStats(rms, 0.010, silThr)

	// 하한/상한선 지정
	var capHi float64
	switch {
	case m.SNRdB >= 18:
		capHi = 500
	case m.SNRdB >= 15:
		capHi = 600
	case m.SNRdB >= 10:
		capHi = 700
	default:
		capHi = 850
	}

	base := 300.0
	snrAdj := clamp((15.0-m.SNRdB)*6.0, 0, 160)
	silAdj := clamp((0.30-m.AvgSilenceSec)*200.0, 0, 150) // 평균 침묵이 0.3s보다 짧으면 증가
	shortAdj := clamp(m.ShortSilenceRatio*100.0, 0, 120)
	m.RawPadMs = base + snrAdj + silAdj + shortAdj
	m.FinalPadMs = int(clamp(m.RawPadMs, 400, capHi))

	minSil := 650.0
	minSil += clamp(m.ShortSilenceRatio*400.0, 0, 300)
	minSil += clamp((m.SNRdB-18.0)*12.0, 0, 120)
	minSil -= clamp((m.AvgSilenceSec-0.45)*500.0, 0, 200)

	// 하한/상한선 지정
	m.SuggestedMinSilenceMs = int(clamp(minSil, 400, 1200))
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

func estimateSilenceStats(rms []float64, hopSec float64, thr float64) (float64, float64) {
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

// whisperClamp : value compare limit
func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// mergeCloseSegments : 너무 짧은 세그먼트는 합침
func mergeCloseSegments(segments []speech.Segment, maxGap float64) []speech.Segment {
	if len(segments) <= 1 {
		return segments
	}

	merged := []speech.Segment{segments[0]}

	for i := 1; i < len(segments); i++ {
		prev := &merged[len(merged)-1]
		curr := segments[i]

		gap := curr.SpeechStartAt - prev.SpeechEndAt

		// 간격이 1.5초 이하면 병합
		if gap <= maxGap {
			prev.SpeechEndAt = curr.SpeechEndAt
		} else {
			merged = append(merged, curr)
		}
	}

	return merged
}

// FindBestMatchingSegments : 자막 구간과 겹치는 모든 VAD 세그먼트 찾기
func FindBestMatchingSegments(speech []speech.Segment, startTime, endTime float64) (float64, []int) {
	if len(speech) == 0 {
		return 0.0, nil
	}

	matchedIndices := make([]int, 0)
	maxOverlap := 0.0

	for i, seg := range speech {
		overlap := GetOverlapRatio(startTime, endTime, seg.SpeechStartAt, seg.SpeechEndAt)
		if overlap > 0 {
			matchedIndices = append(matchedIndices, i)
			if overlap > maxOverlap {
				maxOverlap = overlap
			}
		}
	}

	return maxOverlap, matchedIndices
}

func GetOverlapRatio(segStart, segEnd, vadStart, vadEnd float64) float64 {
	if segEnd <= vadStart || vadEnd <= segStart {
		return 0.0 // 겹침 없음
	}

	overlapStart := math.Max(segStart, vadStart)
	overlapEnd := math.Min(segEnd, vadEnd)
	overlapDuration := overlapEnd - overlapStart

	segDuration := segEnd - segStart
	if segDuration <= 0 {
		return 0.0
	}

	return overlapDuration / segDuration // 자막 구간 대비 겹침 비율
}

// SortSpeechSegments : start_at 오름차순으로 객체 정렬
func SortSpeechSegments(segments []speech.Segment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].SpeechStartAt < segments[j].SpeechStartAt
	})
}

// calculateAvgDuration : 세그먼트 평균 길이 계산
func calculateAvgDuration(segments []speech.Segment) float64 {
	if len(segments) == 0 {
		return 0.0
	}
	var total float64
	for _, seg := range segments {
		total += seg.SpeechEndAt - seg.SpeechStartAt
	}
	return total / float64(len(segments))
}

// 비대칭 패딩 적용: 앞(preMs)과 뒤(postMs)를 다르게 늘리고, 겹치는 세그먼트는 병합
func applyAsymmetricPaddingToSegments(segs []speech.Segment, sr int, preMs, postMs int) []speech.Segment {
	if len(segs) == 0 {
		return segs
	}

	pre := float64(preMs) / 1000.0
	post := float64(postMs) / 1000.0

	// 1) 패딩
	out := make([]speech.Segment, 0, len(segs))
	for _, s := range segs {
		st := s.SpeechStartAt - pre
		en := s.SpeechEndAt + post
		if st < 0 {
			st = 0
		}
		if en < st {
			en = st
		}
		out = append(out, speech.Segment{SpeechStartAt: st, SpeechEndAt: en})
	}

	// 2) 병합
	return mergeOverlappingTimeSegments(out, 0.0) // 0초 간격까지 병합
}

// 시간기준 병합 (gap<=maxGapSec이면 병합)
func mergeOverlappingTimeSegments(segs []speech.Segment, maxGapSec float64) []speech.Segment {
	if len(segs) <= 1 {
		return segs
	}
	SortSpeechSegments(segs)
	merged := []speech.Segment{segs[0]}
	for i := 1; i < len(segs); i++ {
		prev := &merged[len(merged)-1]
		cur := segs[i]
		gap := cur.SpeechStartAt - prev.SpeechEndAt
		if gap <= maxGapSec {
			if cur.SpeechEndAt > prev.SpeechEndAt {
				prev.SpeechEndAt = cur.SpeechEndAt
			}
		} else {
			merged = append(merged, cur)
		}
	}
	return merged
}

// mergeShortGaps : 짧은 무성 병합(히스테리시스의 "release" 완화 효과)
func mergeShortGaps(segs []speech.Segment, maxGapMs int) []speech.Segment {
	if len(segs) <= 1 {
		return segs
	}
	return mergeOverlappingTimeSegments(segs, float64(maxGapMs)/1000.0)
}

// dropShortSpeech : 짧은 발화 제거(히스테리시스의 "enter" 강화 효과)
func dropShortSpeech(segs []speech.Segment, minSpeechMs int) []speech.Segment {
	if len(segs) == 0 {
		return segs
	}
	minDur := float64(minSpeechMs) / 1000.0
	out := out0(segs[:0])
	for _, s := range segs {
		if (s.SpeechEndAt - s.SpeechStartAt) >= minDur {
			out = append(out, s)
		}
	}
	return out
}

// buildEnergyMask : RMS 기반 에너지 마스크 생성 (win/hop 단위 RMS를 샘플 마스크로 확장)
func buildEnergyMask(pcm []float32, sr int, winSec, hopSec float64, energyThr float64) []bool {
	n := len(pcm)
	mask := make([]bool, n)
	if n == 0 {
		return mask
	}

	win := int(winSec * float64(sr))
	hop := int(hopSec * float64(sr))
	if win <= 0 || hop <= 0 || n < win {
		return mask
	}

	// 프레임 RMS 계산
	rms := frameRMS(pcm, sr, winSec, hopSec)
	if len(rms) == 0 {
		return mask
	}

	// 프레임 히트를 샘플 마스크로 펼치기
	s := 0
	for i := 0; i+win <= n && s < len(rms); i += hop {
		if rms[s] > energyThr {
			st, en := i, i+win
			if en > n {
				en = n
			}
			for j := st; j < en; j++ {
				mask[j] = true
			}
		}
		s++
	}
	return mask
}
