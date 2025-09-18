package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/streamer45/silero-vad-go/speech"
)

func VadFilter(wavAudioPath string) (string, error) {
	sd, err := speech.NewDetector(speech.DetectorConfig{
		ModelPath:            "silero_vad.onnx",
		SampleRate:           16000,
		Threshold:            0.5,
		MinSilenceDurationMs: 100,
		SpeechPadMs:          30,
	})
	if err != nil {
		return "", fmt.Errorf("VAD 디텍터 생성 실패: %w", err)
	}
	defer sd.Destroy()

	inputFile, err := os.Open(wavAudioPath)
	if err != nil {
		return "", fmt.Errorf("입력 파일 열기 실패: %w", err)
	}
	defer inputFile.Close()

	// WAV 디코더 생성
	dec := wav.NewDecoder(inputFile)
	if !dec.IsValidFile() {
		return "", fmt.Errorf("유효하지 않은 WAV 파일입니다")
	}

	// PCM 버퍼 읽기
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return "", fmt.Errorf("PCM 버퍼 읽기 실패: %w", err)
	}

	pcmBuf := buf.AsFloat32Buffer()
	sampleRate := buf.Format.SampleRate

	fmt.Printf("샘플레이트: %d Hz\n", sampleRate)
	fmt.Printf("채널 수: %d\n", buf.Format.NumChannels)
	fmt.Printf("총 길이: %.2f초\n", float64(len(pcmBuf.Data))/float64(sampleRate))

	// 음성 구간 탐지
	fmt.Println("음성 구간을 탐지하는 중...")
	segments, err := sd.Detect(pcmBuf.Data)
	if err != nil {
		return "", fmt.Errorf("음성 탐지 실패: %w", err)
	}

	fmt.Printf("탐지된 음성 구간: %d개\n", len(segments))

	// 원본 오디오 데이터를 복사 (전체 길이 유지)
	processedAudio := make([]float32, len(pcmBuf.Data))
	copy(processedAudio, pcmBuf.Data)

	// 음성 구간이 없으면 전체를 무음 처리
	if len(segments) == 0 {
		fmt.Println("⚠️ 음성 구간이 발견되지 않았습니다. 전체를 무음 처리합니다.")
		for i := range processedAudio {
			processedAudio[i] = 0.0
		}
	} else {
		// 음성 구간 외의 모든 곳을 무음 처리
		speechRegions := make([]bool, len(pcmBuf.Data))

		// 음성 구간을 마킹
		for i, segment := range segments {
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

			duration := float64(endSample-startSample) / float64(sampleRate)
			fmt.Printf("음성 구간 %d: %.2fs ~ %.2fs (%.2fs)\n", i+1, segment.SpeechStartAt, segment.SpeechEndAt, duration)
		}

		// 음성이 아닌 구간을 무음 처리
		silencedSamples := 0
		for i := range processedAudio {
			if !speechRegions[i] {
				processedAudio[i] = 0.0 // 무음 처리
				silencedSamples++
			}
		}

		silencedDuration := float64(silencedSamples) / float64(sampleRate)
		speechDuration := float64(len(pcmBuf.Data)-silencedSamples) / float64(sampleRate)
		originalDuration := float64(len(pcmBuf.Data)) / float64(sampleRate)

		fmt.Printf("\n📊 처리 결과:\n")
		fmt.Printf("전체 길이: %.2f초 (유지됨)\n", originalDuration)
		fmt.Printf("음성 구간: %.2f초 (%.1f%%)\n", speechDuration, speechDuration/originalDuration*100)
		fmt.Printf("무음 처리: %.2f초 (%.1f%%)\n", silencedDuration, silencedDuration/originalDuration*100)
	}

	// 출력 파일 경로 생성
	ext := filepath.Ext(wavAudioPath)
	baseName := strings.TrimSuffix(wavAudioPath, ext)
	outputFile := baseName + "_vad_filtered" + ext

	// 출력 파일 생성
	outputF, err := os.Create(outputFile)
	if err != nil {
		return "", fmt.Errorf("출력 파일 생성 실패: %w", err)
	}
	defer outputF.Close()

	// WAV 인코더 생성
	enc := wav.NewEncoder(outputF, sampleRate, 16, buf.Format.NumChannels, 1)

	// Float32를 Int로 변환
	intData := make([]int, len(processedAudio))
	for i, sample := range processedAudio {
		// Float32 (-1.0 ~ 1.0)를 16-bit Int (-32768 ~ 32767)로 변환
		intData[i] = int(sample * 32767)
	}

	// 오디오 버퍼 생성 및 쓰기
	outputBuf := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: buf.Format.NumChannels,
			SampleRate:  sampleRate,
		},
		Data:           intData,
		SourceBitDepth: 16,
	}

	err = enc.Write(outputBuf)
	if err != nil {
		return "", fmt.Errorf("오디오 쓰기 실패: %w", err)
	}

	err = enc.Close()
	if err != nil {
		return "", fmt.Errorf("인코더 닫기 실패: %w", err)
	}

	fmt.Printf("\n✅ 처리 완료!\n")
	fmt.Printf("출력 파일: %s\n", outputFile)
	fmt.Printf("📝 음성이 아닌 구간은 무음 처리되었습니다 (전체 길이 유지)\n")

	return outputFile, nil
}
