package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ExtractAudio(ctx context.Context, inputFile, outputFile string) error {
	// outputFile 확장자가 webm 이면서, input 파일과 output 파일 경로가 같다면 이미 실행했다고 간주
	if filepath.Ext(outputFile) == ".webm" && inputFile == outputFile {
		return fmt.Errorf("audio extractor is already running")
	}

	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file not exist: %s", inputFile)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputFile,
		"-vn",
		"-map_metadata", "-1",
		"-ac", "1",
		"-c:a", "libopus",
		"-b:a", "12k",
		"-application", "voip",
		"-f", "webm",
		"-y",
		outputFile)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run ffmpeg command: %v", err)
	}

	return nil
}

func ExtractAudioToWav(videoPath string) (string, error) {
	var err error

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("비디오 파일을 찾을 수 없습니다: %s", videoPath)
	}

	ext := filepath.Ext(videoPath)
	tempWavPath := strings.TrimSuffix(videoPath, ext) + ".wav"

	log.Printf("오디오 추출 중: %s -> %s\n", videoPath, tempWavPath)

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

	return tempWavPath, nil
}

// ExtractChunkAudio : 감쇠 처리된 오디오에서 특정 구간 추출
func ExtractChunkAudio(filteredAudioPath string, chunk AudioChunk, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, fmt.Sprintf("chunk_%04d.wav", chunk.Index))

	// ffmpeg으로 구간 추출
	cmd := exec.Command("ffmpeg",
		"-i", filteredAudioPath,
		"-ss", fmt.Sprintf("%.3f", chunk.StartSec),
		"-to", fmt.Sprintf("%.3f", chunk.EndSec),
		"-c", "copy",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w, output: %s", err, string(output))
	}

	log.Printf("Extracted chunk #%d: %.2fs - %.2fs (duration: %.2fs = %s) -> %s",
		chunk.Index,
		chunk.StartSec,
		chunk.EndSec,
		chunk.Duration,
		formatDuration(chunk.Duration),
		outputPath,
	)

	return outputPath, nil
}

func formatDuration(seconds float64) string {
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%d분 %02d초", minutes, secs)
}

// ExtractAudioForWhisperChunk : whisper api 요청시 환각이 발생했다면 발생 부분만 재요청하기 위한 chunk audio create
func ExtractAudioForWhisperChunk(ctx context.Context, inputPath, outputPath string, startTime, endTime float64) error {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file not exist: %s", inputPath)
	}

	duration := endTime - startTime
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.2f", startTime),
		"-t", fmt.Sprintf("%.2f", duration),
		"-c", "copy",
		"-y", // 덮어쓰기
		outputPath)

	fmt.Println("extract audio for whisper", "command", cmd.String())
	var stdoutBuf, stderrBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run ffmpeg command: %v", err)
	}

	return nil
}
