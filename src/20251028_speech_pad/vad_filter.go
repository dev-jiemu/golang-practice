package main

import (
	"fmt"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/streamer45/silero-vad-go/speech"
)

const (
	fadeMs      = 15     // 경계 페이드 길이(ms): 10~20ms 권장
	softGateAtt = 0.0178 // -35 dB ≈ 10^(-35/20)
	eps         = 1e-4   // 매우 작은 잔여치 클리핑
)

// VadFilter : 무음구간 처리 필터 호출
func VadFilter(wavFile string, config *speech.DetectorConfig, outputPath string) ([]speech.Segment, error) {
	var err error

	inputFile, err := os.Open(wavFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %v", err)
	}
	defer inputFile.Close()

	decoder := wav.NewDecoder(inputFile)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("audio file is not a valid file")
	}

	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to read PCM buffer: %v", err)
	}

	pcmBuf := buf.AsFloat32Buffer()
	sampleRate := buf.Format.SampleRate
	if buf.Format.NumChannels != 1 {
		return nil, fmt.Errorf("expected mono(1ch), got %dch (preprocess with -ac 1)", buf.Format.NumChannels)
	}

	//metrics := EstimateFileLevelPad(pcmBuf.Data, sampleRate)
	metrics := EstimateFileLevelPadAndMinSilence(pcmBuf.Data, sampleRate)

	fmt.Println("==== VAD Auto-Config ====")
	fmt.Printf("SNR(dB): %.2f | AvgSilence: %.0f ms | ShortSilenceRatio: %.0f%%\n",
		metrics.SNRdB, metrics.AvgSilenceSec*1000.0, metrics.ShortSilenceRatio*100.0)
	fmt.Printf("Pad heuristic(ms): raw=%.1f -> final=%d\n", metrics.RawPadMs, metrics.FinalPadMs)
	fmt.Printf("MinSilence suggestion: %d ms\n", metrics.SuggestedMinSilenceMs)
	fmt.Println("================================")

	cfg := *config
	cfg.SampleRate = sampleRate          // 입력 WAV SR에 맞춤 (사전 리샘플이 없다면 이게 안전)
	cfg.SpeechPadMs = metrics.FinalPadMs // 자동 추정 결과 반영
	cfg.MinSilenceDurationMs = metrics.SuggestedMinSilenceMs

	sd, err := speech.NewDetector(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create speech detector: %v", err)
	}
	defer sd.Destroy()

	segments, err := sd.Detect(pcmBuf.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to detect PCM segments: %v", err)
	}

	// 원본 오디오 데이터를 복사 (전체 길이 유지)
	processedAudio := make([]float32, len(pcmBuf.Data))
	copy(processedAudio, pcmBuf.Data)

	// TODO : 음성구간 없으면 에러처리가 맞을까?
	if len(segments) == 0 {
		for i := range processedAudio { // 음성구간 없으면 전체 무음처리 : 빠른 처리를 위해
			processedAudio[i] = 0.0
		}
	} else {
		speechRegions := make([]bool, len(pcmBuf.Data))

		// 음성 구간 마킹
		for _, segment := range segments {
			startSample := int(segment.SpeechStartAt * float64(sampleRate))
			var endSample int

			if segment.SpeechEndAt > 0 {
				endSample = int(segment.SpeechEndAt * float64(sampleRate))
			} else {
				endSample = len(pcmBuf.Data)
			}

			// 범위 체크
			if startSample < 0 {
				startSample = 0
			}
			if endSample > len(pcmBuf.Data) {
				endSample = len(pcmBuf.Data)
			}
			if startSample >= endSample {
				continue
			}

			// 음성 구간을 true로 마킹
			for j := startSample; j < endSample; j++ {
				speechRegions[j] = true
			}
		}

		// 경계 페이드 적용 먼저
		applyBoundaryFadeOut(processedAudio, speechRegions, sampleRate, fadeMs)

		// 음성이 아닌 구간을 무음 처리
		silencedSamples := 0
		for i := range processedAudio {
			if !speechRegions[i] {
				// 2025.10.16 하드컷 지양
				// processedAudio[i] = 0.0 // 무음 처리
				processedAudio[i] *= softGateAtt // -35 dB 감쇠처리 =
				silencedSamples++
			}
		}

		// 아주 미세한 잔여치는 0으로(비교/노이즈 바닥 안정화용)
		for i, v := range processedAudio {
			if v > -eps && v < eps {
				processedAudio[i] = 0
			}
		}

	}

	var outputFile *os.File
	outputFile, err = os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %v", err)
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
		return nil, fmt.Errorf("failed to write audio: %v", err)
	}

	err = encoder.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close encoder: %v", err)
	}

	return segments, nil
}

// applyBoundaryFadeOut : 경계에서만 1회 페이드아웃(음성→무음 구간 직전 샘플들만 살짝 낮춤)
func applyBoundaryFadeOut(data []float32, speechMask []bool, sampleRate, fadeMs int) {
	n := len(data)
	if n == 0 || fadeMs <= 0 {
		return
	}
	fade := int(float64(sampleRate) * float64(fadeMs) / 1000.0)
	if fade < 1 {
		fade = 1
	}

	for i := 1; i < n; i++ {
		if speechMask[i-1] && !speechMask[i] { // 음성→무음 경계
			end := i - 1
			start := end - fade + 1
			if start < 0 {
				start = 0
			}
			span := end - start + 1
			for k := 0; k < span; k++ {
				// 시작(=경계에서 멀리)≈1 → 끝(=경계 직전)≈0 로 선형 페이드
				alpha := 1.0 - float32(k+1)/float32(span+1)
				data[start+k] *= alpha
			}
		}
	}
}
