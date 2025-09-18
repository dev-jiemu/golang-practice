package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

func ExtractAudioToMP3(vadFilteredWavPath string) (string, error) {
	var finalWebmPath string
	var err error

	if _, err := os.Stat(vadFilteredWavPath); os.IsNotExist(err) {
		return "", fmt.Errorf("파일을 찾을 수 없습니다: %s", finalWebmPath)
	}

	ext := filepath.Ext(vadFilteredWavPath)
	finalWebmPath = strings.TrimSuffix(vadFilteredWavPath, ext) + "_extracted.webm"

	fmt.Printf("오디오 추출 중: %s -> %s\n", vadFilteredWavPath, finalWebmPath)

	webmCmd := exec.Command("ffmpeg",
		"-i", vadFilteredWavPath, // VAD 처리된 WAV
		"-c:a", "libopus",
		"-b:a", "12k",
		"-application", "voip", // 음성 최적화
		"-f", "webm",
		"-y",
		finalWebmPath)

	var stderr bytes.Buffer
	webmCmd.Stderr = &stderr

	err = webmCmd.Run()
	if err != nil {
		return "", fmt.Errorf("FFmpeg 실행 실패: %v\n오류 내용: %s", err, stderr.String())
	}

	// 추출된 파일 크기 확인
	fileInfo, err := os.Stat(finalWebmPath)
	if err != nil {
		return "", fmt.Errorf("추출된 파일 정보를 가져올 수 없습니다: %v", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
	fmt.Printf("추출된 오디오 파일 크기: %.2f MB\n", fileSizeMB)

	return finalWebmPath, nil
}
