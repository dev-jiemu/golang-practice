package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/streamer45/silero-vad-go/speech"
)

type Config struct {
	OpenAIKey string `json:"openai-key"`
}

func LoadConfig() error {
	var err error

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
		log.Fatal("Error loading config file")
	}

	job := &Job{
		OriginalAudioPath: "./sample/e3.mp3",
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
		Threshold: 0.4,
	}

	segments, _, err := VadFilter(config, job)
	if err != nil {
		log.Fatal("Error creating VAD filter: ", err)
	}

	chunkingConfig := ChunkingConfig{
		TargetDurationSec: 60.0,  // 1분
		MinDurationSec:    10.0,  // 10초
		MaxDurationSec:    120.0, // 2분
		OverlapSec:        1.5,   // 1.5초
	}

	chunks := CreateChunksFromVADSegments(segments, chunkingConfig)
	if len(chunks) == 0 {
		log.Fatal("Error creating VAD chunks")
	}

	log.Printf("Created %d chunks from %d VAD segments\n", len(chunks), len(segments))

	outputDir := filepath.Join(filepath.Dir(job.FilteredAudioPath), "chunks")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	log.Printf("Output directory: %s\n", outputDir)

	if err := SaveChunkInfo(chunks, outputDir); err != nil {
		log.Printf("Warning: Failed to save chunk info: %v\n", err)
	}

	for _, chunk := range chunks {
		chunkPath, err := ExtractChunkAudio(job.FilteredAudioPath, chunk, outputDir)
		if err != nil {
			log.Printf("Error extracting chunk #%d: %v\n", chunk.Index, err)
			continue
		}
		log.Printf("✓ Chunk #%d saved: %s\n", chunk.Index, chunkPath)
	}

	log.Printf("All chunks saved to: %s\n", outputDir)
}

// CreateChunksFromVADSegments : VAD 세그먼트를 청크로 변환
func CreateChunksFromVADSegments(segments []speech.Segment, config ChunkingConfig) []AudioChunk {
	if len(segments) == 0 {
		return nil
	}

	var chunks []AudioChunk
	currentChunkStart := segments[0].SpeechStartAt
	currentChunkEnd := segments[0].SpeechEndAt
	currentSegments := []speech.Segment{segments[0]}
	chunkIndex := 0

	for i := 1; i < len(segments); i++ {
		seg := segments[i]
		potentialDuration := seg.SpeechEndAt - currentChunkStart
		log.Printf("portential duration: %f\n", potentialDuration)

		// 최대 길이 초과 시 청크 분리
		if potentialDuration > config.MaxDurationSec {
			chunk := AudioChunk{
				StartSec:    math.Max(0, currentChunkStart-config.OverlapSec),
				EndSec:      currentChunkEnd + config.OverlapSec,
				OverlapSec:  config.OverlapSec,
				VADSegments: currentSegments,
				Index:       chunkIndex,
			}
			chunk.Duration = chunk.EndSec - chunk.StartSec
			chunks = append(chunks, chunk)
			chunkIndex++

			currentChunkStart = seg.SpeechStartAt
			currentChunkEnd = seg.SpeechEndAt
			currentSegments = []speech.Segment{seg}
			continue
		}

		// 세그먼트 간 간격 체크
		gap := seg.SpeechStartAt - currentChunkEnd

		// 간격이 5초 이상이고 최소 길이 충족 시 청크 분리
		if gap > 5.0 && (currentChunkEnd-currentChunkStart) >= config.MinDurationSec {
			chunk := AudioChunk{
				StartSec:    math.Max(0, currentChunkStart-config.OverlapSec),
				EndSec:      currentChunkEnd + config.OverlapSec,
				OverlapSec:  config.OverlapSec,
				VADSegments: currentSegments,
				Index:       chunkIndex,
			}
			chunk.Duration = chunk.EndSec - chunk.StartSec
			chunks = append(chunks, chunk)
			chunkIndex++

			currentChunkStart = seg.SpeechStartAt
			currentChunkEnd = seg.SpeechEndAt
			currentSegments = []speech.Segment{seg}
		} else {
			// 현재 청크에 병합
			currentChunkEnd = seg.SpeechEndAt
			currentSegments = append(currentSegments, seg)
		}
	}

	// 마지막 청크
	if currentChunkEnd-currentChunkStart >= config.MinDurationSec {
		chunk := AudioChunk{
			StartSec:    math.Max(0, currentChunkStart-config.OverlapSec),
			EndSec:      currentChunkEnd + config.OverlapSec,
			OverlapSec:  config.OverlapSec,
			VADSegments: currentSegments,
			Index:       chunkIndex,
		}
		chunk.Duration = chunk.EndSec - chunk.StartSec
		chunks = append(chunks, chunk)
	}

	return chunks
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

func formatDuration(seconds float64) string {
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%d분 %02d초", minutes, secs)
}
