package main

/*
// macOS용 (로컬 개발)
#cgo darwin CFLAGS: -I/usr/local/onnxruntime-osx-arm64-1.18.1/include
#cgo darwin LDFLAGS: -L/usr/local/onnxruntime-osx-arm64-1.18.1/lib -lonnxruntime -Wl,-rpath,/usr/local/onnxruntime-osx-arm64-1.18.1/lib
*/
import "C"

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/streamer45/silero-vad-go/speech"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("사용법: %s <입력파일.mp4> <출력파일.wav>", os.Args[0])
	}

	wavAudioPath, err := ExtractAudioToWav(os.Args[1])
	if err != nil {
		log.Fatalf("Error extracting audio: %v", err)
	}

	fmt.Printf("result wav audio path : %s\n", wavAudioPath)

	config := &speech.DetectorConfig{
		ModelPath:            "silero_vad.onnx",
		SampleRate:           16000,
		Threshold:            0.5,
		MinSilenceDurationMs: 500,
		SpeechPadMs:          100,
	}
	_, err = VadFilter(wavAudioPath, config, os.Args[2])
}

func ExtractAudioToWav(videoPath string) (string, error) {
	var err error

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("비디오 파일을 찾을 수 없습니다: %s", videoPath)
	}

	ext := filepath.Ext(videoPath)
	tempWavPath := strings.TrimSuffix(videoPath, ext) + ".wav"

	fmt.Printf("오디오 추출 중: %s -> %s\n", videoPath, tempWavPath)

	wavCmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vn",
		"-c:a", "pcm_s16le", // VAD용 16-bit PCM
		"-ar", "16000", // VAD용 16kHz
		"-ac", "1", // VAD는 모노가 더 정확
		"-map_metadata", "-1",
		"-f", "wav",
		"-y",
		tempWavPath) // 임시 WAV 파일

	var stderr bytes.Buffer
	wavCmd.Stderr = &stderr

	err = wavCmd.Run()
	if err != nil {
		return "", fmt.Errorf("FFmpeg 실행 실패: %v\n오류 내용: %s", err, stderr.String())
	}

	// 추출된 파일 크기 확인
	fileInfo, err := os.Stat(tempWavPath)
	if err != nil {
		return "", fmt.Errorf("추출된 파일 정보를 가져올 수 없습니다: %v", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
	fmt.Printf("추출된 오디오 파일 크기: %.2f MB\n", fileSizeMB)

	return tempWavPath, nil
}
