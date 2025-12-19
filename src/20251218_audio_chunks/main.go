package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/streamer45/silero-vad-go/speech"
)

type Config struct {
	OpenAIKey string `json:"openai-key"`
}

type ChunkResult struct {
	Chunk              AudioChunk
	ChunkPath          string
	WhisperResponse    *WhisperResponse
	TranscriptionError error
	Error              error
	Duration           time.Duration
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

	// 리소스 모니터 초기화
	monitor := NewResourceMonitor()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 500ms 마다 리소스 수집
	monitor.StartMonitoring(ctx, 500*time.Millisecond)
	monitor.SetTotalChunks(len(chunks))

	// 청크 작업을 전달할 채널과 결과를 받을 채널
	chunkJobs := make(chan AudioChunk, len(chunks))
	results := make(chan ChunkResult, len(chunks))

	// 워커 수 (CPU 코어 수만큼 또는 원하는 수로 설정)
	numWorkers := 3
	if len(chunks) < numWorkers {
		numWorkers = len(chunks)
	}

	// CPU 코어 수 기반 자동 조정 옵션
	// numWorkers = runtime.NumCPU() / 2 // CPU 코어의 절반만 사용

	log.Printf("🚀 Starting chunk processing with %d workers (CPU cores: %d)\n", numWorkers, runtime.NumCPU())

	translator := NewTranslatorWhisper(segments)

	// 워커 고루틴 시작
	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			monitor.WorkerStart(workerID)
			defer monitor.WorkerEnd(workerID)

			for chunk := range chunkJobs {
				chunkStartTime := time.Now()
				log.Printf("[Worker %d] Processing chunk #%d (%.2fs - %.2fs)\n",
					workerID, chunk.Index, chunk.StartSec, chunk.EndSec)

				result := ChunkResult{
					Chunk:    chunk,
					Duration: 0,
				}

				// 1. 청크 오디오 파일 생성
				chunkPath, err := ExtractChunkAudio(job.WavAudioPath, chunk, outputDir)
				result.ChunkPath = chunkPath

				if err != nil {
					result.Error = err
					result.Duration = time.Since(chunkStartTime)
					monitor.ChunkProcessed(false, result.Duration)
					results <- result
					continue
				}

				log.Printf("[Worker %d] Chunk #%d file created, calling Whisper API...\n", workerID, chunk.Index)

				// 2. Whisper API 호출 (webm 변환 포함)
				webmPath := strings.TrimSuffix(chunkPath, filepath.Ext(chunkPath)) + ".webm"

				// WAV -> WebM 변환
				extractErr := ExtractAudio(ctx, chunkPath, webmPath)
				if extractErr != nil {
					result.TranscriptionError = fmt.Errorf("webm conversion failed: %w", extractErr)
					result.Duration = time.Since(chunkStartTime)
					monitor.ChunkProcessed(false, result.Duration)
					results <- result
					continue
				}

				// Whisper API 호출
				whisperResp, whisperErr := translator.CallWhisperApi(ctx, webmPath, job)
				result.WhisperResponse = whisperResp
				result.TranscriptionError = whisperErr

				if whisperErr != nil {
					log.Printf("[Worker %d] ⚠️  Chunk #%d Whisper API failed: %v\n", workerID, chunk.Index, whisperErr)
				} else {
					log.Printf("[Worker %d] ✓ Chunk #%d transcription completed (%d segments)\n",
						workerID, chunk.Index, len(whisperResp.Segments))
				}

				result.Duration = time.Since(chunkStartTime)
				monitor.ChunkProcessed(whisperErr == nil && err == nil, result.Duration)

				results <- result
			}
		}(w)
	}

	// 모든 청크를 작업 채널에 전송
	for _, chunk := range chunks {
		chunkJobs <- chunk
	}
	close(chunkJobs) // 더 이상 작업이 없음을 알림

	// 결과 수집 + 주기적 진행상황 출력
	successChunks := make([]ChunkResult, 0)
	failedChunks := make([]ChunkResult, 0)

	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	receivedCount := 0
	for receivedCount < len(chunks) {
		select {
		case result := <-results:
			receivedCount++
			if result.Error != nil || result.TranscriptionError != nil {
				log.Printf("✗ Chunk #%d failed (took %s)\n",
					result.Chunk.Index, result.Duration.Round(time.Millisecond))
				if result.Error != nil {
					log.Printf("  File error: %v\n", result.Error)
				}
				if result.TranscriptionError != nil {
					log.Printf("  Whisper error: %v\n", result.TranscriptionError)
				}
				failedChunks = append(failedChunks, result)
			} else {
				log.Printf("✓ Chunk #%d completed (took %s): %s\n",
					result.Chunk.Index, result.Duration.Round(time.Millisecond), result.ChunkPath)
				successChunks = append(successChunks, result)
			}

		case <-progressTicker.C:
			monitor.PrintProgress()
		}
	}

	// 최종 모니터링 요약
	monitor.PrintSummary()

	// 최종 결과 출력
	log.Printf("===== Chunk Processing Summary =====\n")
	log.Printf("Total chunks: %d\n", len(chunks))
	log.Printf("Success: %d\n", len(successChunks))
	log.Printf("Failed: %d\n", len(failedChunks))

	if len(failedChunks) > 0 {
		log.Printf("Failed chunks:\n")
		for _, failed := range failedChunks {
			log.Printf("  - Chunk #%d\n", failed.Chunk.Index)
			if failed.Error != nil {
				log.Printf("    Error: %v\n", failed.Error)
			}
			if failed.TranscriptionError != nil {
				log.Printf("    Transcription: %v\n", failed.TranscriptionError)
			}
		}
	}

	// 3. 타임스탬프 보정 및 자막 통합
	if len(successChunks) > 0 {
		log.Println("===== Merging Transcriptions =====")
		allSubtitles := MergeChunkTranscriptions(successChunks, translator)

		// 4. JSON 저장
		outputJSON := filepath.Join(outputDir, "transcription.json")
		if err := SaveTranscriptionJSON(allSubtitles, outputJSON); err != nil {
			log.Printf("❌ Failed to save transcription JSON: %v\n", err)
		} else {
			log.Printf("✅ Transcription saved to: %s\n", outputJSON)
			log.Printf("   Total subtitle segments: %d\n", len(allSubtitles))
		}
	} else {
		log.Println("\n⚠️  No successful chunks to merge")
	}
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

// MergeChunkTranscriptions : 청크별 Whisper 응답을 타임스탬프 보정하여 통합
func MergeChunkTranscriptions(chunkResults []ChunkResult, translator *TranslatorWhisper) []SubtitleSegment {
	allSubtitles := make([]SubtitleSegment, 0)

	for _, result := range chunkResults {
		if result.WhisperResponse == nil {
			continue
		}

		// 청크 시작 시간 (타임스탬프 오프셋)
		timeOffset := result.Chunk.StartSec

		log.Printf("Processing chunk #%d (offset: %.2fs, segments: %d)\n",
			result.Chunk.Index, timeOffset, len(result.WhisperResponse.Segments))

		// Whisper 응답을 자막 형식으로 변환
		subtitles := translator.ConvertWhisperResponse(result.WhisperResponse)

		// 타임스탬프 보정: 청크 시작 시간을 더함
		for i := range subtitles {
			subtitles[i].StartTime += timeOffset
			subtitles[i].EndTime += timeOffset

			// SentenceFrames의 타임스탬프도 보정
			for j := range subtitles[i].SentenceFrames {
				subtitles[i].SentenceFrames[j].WordStartTime += timeOffset
				subtitles[i].SentenceFrames[j].WordEndTime += timeOffset
			}
		}

		allSubtitles = append(allSubtitles, subtitles...)
	}

	// 시간순으로 정렬
	SortSubtitleSegment(allSubtitles)

	// 인덱스 재정렬
	for i := range allSubtitles {
		allSubtitles[i].Idx = i
	}

	log.Printf("Total merged subtitles: %d\n", len(allSubtitles))

	return allSubtitles
}

// SaveTranscriptionJSON : 자막 데이터를 JSON 파일로 저장
func SaveTranscriptionJSON(subtitles []SubtitleSegment, outputPath string) error {
	data, err := json.MarshalIndent(subtitles, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON marshal failed: %w", err)
	}

	err = os.WriteFile(outputPath, data, 0644)
	if err != nil {
		return fmt.Errorf("file write failed: %w", err)
	}

	return nil
}
