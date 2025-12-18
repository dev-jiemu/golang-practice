package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/streamer45/silero-vad-go/speech"
)

type Config struct {
	OpenAIKey string `json:"openai-key"`
}

func LoadConfig() error {
	WhisperConfig = &Config{}

	configFile, err := os.Open("./config.json")
	if err != nil {
		return fmt.Errorf("Error opening config file: %s", err)
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(WhisperConfig)
	if err != nil {
		return fmt.Errorf("Error parsing config file: %s", err)
	}

	if WhisperConfig.OpenAIKey == "" {
		return fmt.Errorf("No openai-key found in config file")
	}

	return nil
}

func main() {
	if LoadConfig() != nil {
		log.Fatal("Error loading config")
	}

	job := &Job{
		OriginalAudioPath: "./sample/e3.mp4",
		RId:               "jiemu-test",
	}

	wavPath, err := ExtractAudioToWav(job.OriginalAudioPath)
	if err != nil {
		log.Fatalf("Error extracting audio from wav: %s", err)
	}
	job.WavAudioPath = wavPath

	ext := filepath.Ext(wavPath)
	job.FilteredAudioPath = strings.TrimSuffix(wavPath, ext) + "_filtered.wav"

	config := &speech.DetectorConfig{
		ModelPath: "silero_vad.onnx",
		Threshold: 0.5,
	}

	//segments, _, totalDuration, err := VadFilter(config, job)
	segments, totalDuration, err := VadFilterDetectOnly(config, job)
	if err != nil {
		log.Fatal("Error creating VAD filter: ", err)
	}

	chunkingConfig := ChunkingConfig{
		MinDurationSec: 10.0,  // 10초
		MaxDurationSec: 600.0, // 10분
		OverlapSec:     1.5,   // 1.5초
	}

	chunks := CreateAudioChunks(segments, chunkingConfig, totalDuration)

	log.Printf("Created %d chunks from %d VAD segments\n", len(chunks), len(segments))

	outputDir := filepath.Join(filepath.Dir(job.WavAudioPath), "chunks")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	log.Printf("Output directory: %s\n", outputDir)

	if err := SaveChunkInfo(chunks, outputDir); err != nil {
		log.Printf("Warning: Failed to save chunk info: %v\n", err)
	}

	for _, chunk := range chunks {
		chunkPath, err := ExtractChunkAudio(job.WavAudioPath, chunk, outputDir)
		if err != nil {
			log.Printf("Error extracting chunk #%d: %v\n", chunk.Index, err)
			continue
		}
		log.Printf("✓ Chunk #%d saved: %s\n", chunk.Index, chunkPath)
	}

	log.Printf("All chunks saved to: %s\n", outputDir)
}

func CreateAudioChunks(vadSegments []speech.Segment, config ChunkingConfig, totalDurationSec float64) []AudioChunk {
	if len(vadSegments) == 0 {
		return nil
	}

	chunks := make([]AudioChunk, 0)
	currentChunk := AudioChunk{
		StartSec:    0, // 파일 처음부터 시작
		VADSegments: []speech.Segment{},
		Index:       0,
	}

	for i, seg := range vadSegments {
		// 현재 청크에 이 세그먼트를 추가했을 때의 duration
		potentialEndSec := seg.SpeechEndAt
		potentialDuration := potentialEndSec - currentChunk.StartSec

		// MaxDuration을 초과하면 청크 분할
		if potentialDuration > config.MaxDurationSec && len(currentChunk.VADSegments) > 0 {
			// 현재 청크 마무리: 마지막 VAD 세그먼트가 끝나는 지점까지
			lastSegEnd := currentChunk.VADSegments[len(currentChunk.VADSegments)-1].SpeechEndAt
			currentChunk.EndSec = lastSegEnd
			currentChunk.Duration = currentChunk.EndSec - currentChunk.StartSec
			chunks = append(chunks, currentChunk)

			// 새 청크 시작: 이전 청크가 끝난 바로 다음부터 (겹침 없음)
			currentChunk = AudioChunk{
				StartSec:    lastSegEnd, // 이전 청크 끝 = 다음 청크 시작
				VADSegments: []speech.Segment{seg},
				Index:       len(chunks),
			}
		} else {
			// 현재 청크에 세그먼트 추가
			currentChunk.VADSegments = append(currentChunk.VADSegments, seg)
		}

		// 마지막 세그먼트면 청크 저장
		if i == len(vadSegments)-1 {
			currentChunk.EndSec = totalDurationSec // 파일 끝까지
			currentChunk.Duration = currentChunk.EndSec - currentChunk.StartSec
			chunks = append(chunks, currentChunk)
		}
	}

	// MinDuration 체크: 너무 짧은 청크는 이전 청크와 병합
	if len(chunks) > 1 {
		chunks = mergeShortChunks(chunks, config.MinDurationSec)
	}

	return chunks
}

// mergeShortChunks : MinDuration보다 짧은 청크를 이전 청크와 병합
func mergeShortChunks(chunks []AudioChunk, minDuration float64) []AudioChunk {
	merged := make([]AudioChunk, 0, len(chunks))

	for i, chunk := range chunks {
		if chunk.Duration < minDuration && i > 0 {
			// 이전 청크와 병합
			prev := &merged[len(merged)-1]
			prev.EndSec = chunk.EndSec
			prev.Duration = prev.EndSec - prev.StartSec
			prev.VADSegments = append(prev.VADSegments, chunk.VADSegments...)
		} else {
			chunk.Index = len(merged)
			merged = append(merged, chunk)
		}
	}

	return merged
}

// SaveChunkInfo : 청크 정보를 텍스트 파일로 저장
func SaveChunkInfo(chunks []AudioChunk, outputDir string) error {
	infoPath := filepath.Join(outputDir, "chunks_info.txt")
	f, err := os.Create(infoPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "Total Chunks: %d\n", len(chunks))
	fmt.Fprintf(f, "=====================================\n\n")

	for _, chunk := range chunks {
		fmt.Fprintf(f, "Chunk #%d:\n", chunk.Index)
		fmt.Fprintf(f, "  Time Range: %.3fs - %.3fs\n", chunk.StartSec, chunk.EndSec)
		fmt.Fprintf(f, "  Duration: %.2fs = %s\n", chunk.Duration, formatDuration(chunk.Duration))
		fmt.Fprintf(f, "  Overlap: %.2fs\n", chunk.OverlapSec)
		fmt.Fprintf(f, "  VAD Segments: %d\n", len(chunk.VADSegments))

		if len(chunk.VADSegments) > 0 {
			fmt.Fprintf(f, "  Speech Segments:\n")
			for i, seg := range chunk.VADSegments {
				fmt.Fprintf(f, "    [%d] %.3fs - %.3fs (%.2fs)\n",
					i,
					seg.SpeechStartAt,
					seg.SpeechEndAt,
					seg.SpeechEndAt-seg.SpeechStartAt,
				)
			}
		}
		fmt.Fprintf(f, "\n")
	}

	log.Printf("Chunk info saved to: %s\n", infoPath)
	return nil
}
