package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/streamer45/silero-vad-go/speech"
)

// mergeCloseSegments - 가까운 구간 병합
func mergeCloseSegments(segments []speech.Segment, maxGap float64) []speech.Segment {
	if len(segments) == 0 {
		return segments
	}

	merged := make([]speech.Segment, 0, len(segments))
	current := segments[0]

	for i := 1; i < len(segments); i++ {
		gap := segments[i].SpeechStartAt - current.SpeechEndAt
		if gap <= maxGap {
			fmt.Printf("🔗 병합: [%.2f-%.2f] + [%.2f-%.2f] (간격: %.2fs)\n",
				current.SpeechStartAt, current.SpeechEndAt,
				segments[i].SpeechStartAt, segments[i].SpeechEndAt, gap)
			current.SpeechEndAt = segments[i].SpeechEndAt
		} else {
			merged = append(merged, current)
			current = segments[i]
		}
	}
	merged = append(merged, current)

	return merged
}

// filterShortSegments - 짧은 구간 제거
func filterShortSegments(segments []speech.Segment, minDuration float64) []speech.Segment {
	if len(segments) == 0 {
		return segments
	}

	filtered := make([]speech.Segment, 0, len(segments))
	for _, seg := range segments {
		duration := seg.SpeechEndAt - seg.SpeechStartAt
		if duration >= minDuration {
			filtered = append(filtered, seg)
		} else {
			fmt.Printf("🚫 짧은 구간 제거 (%.2fs < %.2fs): %.2f ~ %.2f\n",
				duration, minDuration, seg.SpeechStartAt, seg.SpeechEndAt)
		}
	}
	return filtered
}

// removeIsolatedNoise - 고립된 노이즈 제거
// 앞뒤로 긴 무음이 있는 짧은 구간은 노이즈로 판단
func removeIsolatedNoise(segments []speech.Segment, minIsolationGap, maxNoiseDuration float64) []speech.Segment {
	if len(segments) <= 1 {
		return segments
	}

	cleaned := make([]speech.Segment, 0, len(segments))

	for i, seg := range segments {
		duration := seg.SpeechEndAt - seg.SpeechStartAt

		// 긴 구간은 무조건 유지
		if duration > maxNoiseDuration {
			cleaned = append(cleaned, seg)
			continue
		}

		// 첫/마지막 구간은 한쪽 간격만 확인
		if i == 0 {
			if len(segments) > 1 && segments[1].SpeechStartAt-seg.SpeechEndAt < minIsolationGap {
				cleaned = append(cleaned, seg)
			} else {
				fmt.Printf("🚫 고립 노이즈 제거 (시작, %.2fs): %.2f ~ %.2f\n",
					duration, seg.SpeechStartAt, seg.SpeechEndAt)
			}
			continue
		}

		if i == len(segments)-1 {
			if seg.SpeechStartAt-segments[i-1].SpeechEndAt < minIsolationGap {
				cleaned = append(cleaned, seg)
			} else {
				fmt.Printf("🚫 고립 노이즈 제거 (끝, %.2fs): %.2f ~ %.2f\n",
					duration, seg.SpeechStartAt, seg.SpeechEndAt)
			}
			continue
		}

		// 중간 구간: 앞뒤 간격 모두 확인
		gapBefore := seg.SpeechStartAt - segments[i-1].SpeechEndAt
		gapAfter := segments[i+1].SpeechStartAt - seg.SpeechEndAt

		if gapBefore >= minIsolationGap && gapAfter >= minIsolationGap {
			fmt.Printf("🚫 고립 노이즈 제거 (%.2fs, 앞:%.2fs 뒤:%.2fs): %.2f ~ %.2f\n",
				duration, gapBefore, gapAfter, seg.SpeechStartAt, seg.SpeechEndAt)
		} else {
			cleaned = append(cleaned, seg)
		}
	}

	return cleaned
}

// 간단한 선형 리샘플러 (고품질 필요시 추후 대체)
func resampleLinearFloat32(in []float32, inSR, outSR int) []float32 {
	if inSR == outSR || len(in) == 0 {
		out := make([]float32, len(in))
		copy(out, in)
		return out
	}
	ratio := float64(outSR) / float64(inSR)
	outLen := int(math.Round(float64(len(in)) * ratio))
	if outLen <= 1 {
		return []float32{}
	}
	out := make([]float32, outLen)
	for i := 0; i < outLen; i++ {
		srcPos := float64(i) / ratio
		j := int(math.Floor(srcPos))
		t := float32(srcPos - float64(j))
		if j >= len(in)-1 {
			out[i] = in[len(in)-1]
		} else {
			a := in[j]
			b := in[j+1]
			out[i] = a + (b-a)*t
		}
	}
	return out
}

func clipAndQuantizeInt16(x float32) int {
	// 클리핑 + 라운딩
	if x > 1.0 {
		x = 1.0
	} else if x < -1.0 {
		x = -1.0
	}
	v := int(math.Round(float64(x * 32767.0)))
	if v > 32767 {
		v = 32767
	}
	if v < -32768 {
		v = -32768
	}
	return v
}

func VadFilter(wavAudioPath string) ([]speech.Segment, string, error) {
	// 1) VAD 디텍터(보수적 파라미터)
	sd, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:            "silero_vad.onnx",
		SampleRate:           16000, // 고정
		Threshold:            0.4,   // 0.4~0.6 A/B
		MinSilenceDurationMs: 700,   // 300~800 A/B
		SpeechPadMs:          200,   // 50~200 A/B
	})
	if err != nil {
		return nil, "", fmt.Errorf("VAD 디텍터 생성 실패: %w", err)
	}
	defer sd.Destroy()

	inputFile, err := os.Open(wavAudioPath)
	if err != nil {
		return nil, "", fmt.Errorf("입력 파일 열기 실패: %w", err)
	}
	defer inputFile.Close()

	dec := wav.NewDecoder(inputFile)
	if !dec.IsValidFile() {
		return nil, "", fmt.Errorf("유효하지 않은 WAV 파일입니다")
	}
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, "", fmt.Errorf("PCM 버퍼 읽기 실패: %w", err)
	}

	pcmBuf := buf.AsFloat32Buffer()
	inSR := buf.Format.SampleRate
	ch := buf.Format.NumChannels

	// 길이 계산은 채널 고려
	inDuration := float64(len(pcmBuf.Data)) / float64(inSR*ch)
	fmt.Printf("입력: %d Hz, %dch, 길이: %.2fs\n", inSR, ch, inDuration)

	// 3) (모노 가드) ffmpeg에서 -ac 1이 보장되어야 함
	if ch != 1 {
		return nil, "", fmt.Errorf("모노(1ch)가 아닙니다: got %dch (ffmpeg 전처리 -ac 1 확인 필요)", ch)
	}
	mono := pcmBuf.Data

	// 16k로 리샘플 (이미 16000이면 패스)
	mono16k := resampleLinearFloat32(mono, inSR, 16000)
	if len(mono16k) == 0 {
		return nil, "", fmt.Errorf("리샘플 결과가 비어 있습니다")
	}

	fmt.Printf("정규화 후: 16000 Hz, 1ch, 길이: %.2fs\n", float64(len(mono16k))/16000.0)

	// 4) VAD 실행 (모노16k 기준)
	fmt.Println("음성 구간을 탐지하는 중...")
	segments, err := sd.Detect(mono16k)
	if err != nil {
		return nil, "", fmt.Errorf("음성 탐지 실패: %w", err)
	}
	fmt.Printf("탐지된 음성 구간: %d개\n", len(segments))

	if len(segments) == 0 {
		fmt.Println("⚠️ 음성 구간이 없습니다.")
		// 전체를 무음 처리하고 바로 저장
		processed := make([]float32, len(mono16k))
		return saveProcessedAudio(processed, wavAudioPath, segments)
	}

	// ========== 필터링 파이프라인 (최적 순서) ==========

	// 1단계: 가까운 구간 병합 (0.8초 이하 간격)
	fmt.Println("\n🔗 1단계: 가까운 구간 병합...")
	segments = mergeCloseSegments(segments, 0.8)
	fmt.Printf("   결과: %d개\n", len(segments))

	// 2단계: 짧은 구간 제거 (300ms 미만)
	fmt.Println("\n🔪 2단계: 짧은 구간 제거...")
	segments = filterShortSegments(segments, 0.3)
	fmt.Printf("   결과: %d개\n", len(segments))

	// 3단계: 고립된 노이즈 제거 (1.5초 이상 고립된 0.6초 미만 구간)
	fmt.Println("\n🎯 3단계: 고립 노이즈 제거...")
	segments = removeIsolatedNoise(segments, 1.5, 0.6)
	fmt.Printf("   최종: %d개\n", len(segments))

	// ===================================================

	// 음성 구간 출력
	fmt.Println("\n📋 최종 음성 구간:")
	totalSpeechDuration := 0.0
	for i, sg := range segments {
		duration := sg.SpeechEndAt - sg.SpeechStartAt
		totalSpeechDuration += duration
		fmt.Printf("   %d: %.2fs ~ %.2fs (%.2fs)\n",
			i+1, sg.SpeechStartAt, sg.SpeechEndAt, duration)
	}

	// 긴 무음(≥1초)만 제거하여 오디오 처리
	processed := make([]float32, len(mono16k))
	copy(processed, mono16k)

	speechMask := make([]bool, len(processed))
	for _, sg := range segments {
		start := int(sg.SpeechStartAt * 16000.0)
		end := int(sg.SpeechEndAt * 16000.0)
		if start < 0 {
			start = 0
		}
		if end > len(processed) {
			end = len(processed)
		}
		for j := start; j < end; j++ {
			speechMask[j] = true
		}
	}

	// 1초 이상의 긴 무음만 제거
	const longSilenceThreshold = 1.0
	silenced := 0
	inSilence := false
	silenceStart := 0

	for i := 0; i < len(processed); i++ {
		if !speechMask[i] {
			if !inSilence {
				inSilence = true
				silenceStart = i
			}
			// 무음 구간 끝에서 처리
			if i == len(processed)-1 || speechMask[i+1] {
				length := float64(i-silenceStart+1) / 16000.0
				if length >= longSilenceThreshold {
					for j := silenceStart; j <= i; j++ {
						processed[j] = 0
						silenced++
					}
					fmt.Printf("   🔇 무음 제거: %.2fs ~ %.2fs (%.2fs)\n",
						float64(silenceStart)/16000.0, float64(i)/16000.0, length)
				}
				inSilence = false
			}
		}
	}

	// 통계 출력
	totalDur := float64(len(processed)) / 16000.0
	silencedDur := float64(silenced) / 16000.0
	speechDur := totalSpeechDuration
	fmt.Printf("\n📊 처리 결과:\n")
	fmt.Printf("   전체:     %.2fs\n", totalDur)
	fmt.Printf("   음성:     %.2fs (%.1f%%)\n", speechDur, speechDur/totalDur*100.0)
	fmt.Printf("   무음제거: %.2fs (%.1f%%)\n", silencedDur, silencedDur/totalDur*100.0)

	return saveProcessedAudio(processed, wavAudioPath, segments)
}

// saveProcessedAudio - 처리된 오디오 저장 (공통 함수)
func saveProcessedAudio(processed []float32, originalPath string, segments []speech.Segment) ([]speech.Segment, string, error) {
	ext := filepath.Ext(originalPath)
	base := strings.TrimSuffix(originalPath, ext)
	outputFile := base + "_vad_filtered" + ext

	outputF, err := os.Create(outputFile)
	if err != nil {
		return nil, "", fmt.Errorf("출력 파일 생성 실패: %w", err)
	}
	defer outputF.Close()

	enc := wav.NewEncoder(outputF, 16000, 16, 1, 1)

	intData := make([]int, len(processed))
	for i, s := range processed {
		intData[i] = clipAndQuantizeInt16(s)
	}
	outBuf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  16000,
		},
		Data:           intData,
		SourceBitDepth: 16,
	}
	if err := enc.Write(outBuf); err != nil {
		return nil, "", fmt.Errorf("오디오 쓰기 실패: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, "", fmt.Errorf("인코더 닫기 실패: %w", err)
	}

	fmt.Printf("\n✅ 완료: %s\n", outputFile)
	return segments, outputFile, nil
}
