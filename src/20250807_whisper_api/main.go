package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	OpenAIKey string `json:"openai-key"`
}

type Word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Segment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

type WhisperResponse struct {
	Task     string    `json:"task"`
	Language string    `json:"language"`
	Duration float64   `json:"duration"`
	Text     string    `json:"text"`
	Words    []Word    `json:"words"` // 이거 완전 뽀개져서 오네...ㅋㅋㅋ
	Segments []Segment `json:"segments"`
}

func loadConfig() (*Config, error) {
	var config *Config
	var err error

	configFile, err := os.Open("./config.json")
	if err != nil {
		return nil, fmt.Errorf("Error opening config file: %s", err)
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("Error parsing config file: %s", err)
	}

	if config.OpenAIKey == "" {
		return nil, fmt.Errorf("No openai-key found in config file")
	}

	return config, nil
}

func transcribeAudio(config *Config, filePath string) {
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		fmt.Printf("파일을 찾을 수 없습니다: %s\n", filePath)
	}
	if err != nil {
		fmt.Printf("파일 정보를 가져올 수 없습니다: %v\n", err)
	}

	// 파일 크기 체크 (25MB = 25 * 1024 * 1024 bytes)
	const maxFileSize = 25 * 1024 * 1024
	fileSize := fileInfo.Size()
	fileSizeMB := float64(fileSize) / (1024 * 1024)

	fmt.Printf("파일 크기: %.2f MB\n", fileSizeMB)

	if fileSize > maxFileSize {
		fmt.Printf("파일 크기가 너무 큽니다 (%.2f MB). OpenAI Whisper API는 최대 25MB까지만 지원합니다. 파일을 압축하거나 분할해주세요\n", fileSizeMB)
	}

	// 파일 열기
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("파일을 열 수 없습니다: %v\n", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Printf("form file 생성 실패: %v\n", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Printf("파일 복사 실패: %v", err)
	}

	// 다른 필드들 추가
	writer.WriteField("model", "whisper-1") // 시간 문제때문에 이거로 해야함
	writer.WriteField("response_format", "srt")
	// writer.WriteField("timestamp_granularities[]", "word")

	err = writer.Close()
	if err != nil {
		fmt.Printf("multipart writer close fail : %v\n", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &body)
	if err != nil {
		fmt.Printf("HTTP 요청 생성 실패: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+config.OpenAIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("HTTP 요청 실행 실패: %v\n", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("응답 읽기 실패: %v\n", err)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API 오류 (상태코드: %d): %s\n", resp.StatusCode, string(responseBody))
	}

	//var whisperResponse WhisperResponse
	//err = json.Unmarshal(responseBody, &whisperResponse)
	//if err != nil {
	//	fmt.Printf("JSON 파싱 실패: %v\n", err)
	//}
	//
	//fmt.Printf("result : [%+v]\n", whisperResponse)
	//
	//jsonData, err := json.MarshalIndent(whisperResponse, "", "  ")
	//if err != nil {
	//	fmt.Printf("JSON 마샬링 실패: %v\n", err)
	//	return
	//}
	//
	//filename := "whisper_output.json"
	//err = os.WriteFile(filename, jsonData, 0644)
	//if err != nil {
	//	fmt.Printf("파일 저장 실패: %v\n", err)
	//	return
	//}

	// JSON으로 받았다면
	//saveWhisperOutput(responseBody, "json")

	// SRT로 받았다면
	saveWhisperOutput(responseBody, "srt")
}

func extractAudioToMP3(videoPath string) (string, error) {
	var audioPath string
	var err error

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("비디오 파일을 찾을 수 없습니다: %s", videoPath)
	}

	ext := filepath.Ext(videoPath)
	audioPath = strings.TrimSuffix(videoPath, ext) + "_extracted.webm"

	fmt.Printf("오디오 추출 중: %s -> %s\n", videoPath, audioPath)

	/*
		*** 이렇게 해버리면 약 30분 영상밖에 안돼...ㅠ..
		// FFmpeg 명령어 실행
		// -i: 입력 파일
		// -vn: 비디오 스트림 제외 (오디오만)
		// -ar 44100: 샘플링 레이트 44.1kHz
		// -ac 2: 스테레오 채널
		// -b:a 128k: 비트레이트 128kbps (파일 크기 최적화)
		// -y: 출력 파일이 존재하면 덮어쓰기
		cmd := exec.Command("ffmpeg",
			"-i", videoPath,
			"-vn",
			"-ar", "44100",
			"-ac", "2",
			"-b:a", "128k",
			"-y",
			audioPath)
	*/

	// FFmpeg 명령어 실행 (Opus 코덱 + WebM 컨테이너)
	// -i: 입력 파일
	// -vn: 비디오 스트림 제외 (오디오만)
	// -map_metadata -1: 메타데이터 제거 (파일 크기 절약)
	// -ac 1: 모노 채널 (음성은 모노로도 충분)
	// -c:a libopus: Opus 코덱 사용
	// -b:a 12k: 비트레이트 12kbps (음성 최적화)
	// -application voip: 음성 통화 최적화 모드
	// -f webm: WebM 컨테이너 강제 지정
	// -y: 출력 파일이 존재하면 덮어쓰기
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vn",
		"-map_metadata", "-1",
		"-ac", "1",
		"-c:a", "libopus",
		"-b:a", "12k",
		"-application", "voip",
		"-f", "webm",
		"-y",
		audioPath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("FFmpeg 실행 실패: %v\n오류 내용: %s", err, stderr.String())
	}

	// 추출된 파일 크기 확인
	fileInfo, err := os.Stat(audioPath)
	if err != nil {
		return "", fmt.Errorf("추출된 파일 정보를 가져올 수 없습니다: %v", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
	fmt.Printf("추출된 오디오 파일 크기: %.2f MB\n", fileSizeMB)

	return audioPath, nil
}

func saveWhisperOutput(responseBody []byte, responseFormat string) {
	if responseFormat == "json" || responseFormat == "verbose_json" {
		// ✅ JSON 파싱
		var whisperResponse WhisperResponse
		err := json.Unmarshal(responseBody, &whisperResponse)
		if err != nil {
			fmt.Printf("JSON 파싱 실패: %v\n", err)
			return
		}

		// pretty JSON으로 저장
		jsonData, err := json.MarshalIndent(whisperResponse, "", "  ")
		if err != nil {
			fmt.Printf("JSON 마샬링 실패: %v\n", err)
			return
		}

		err = os.WriteFile("whisper_output.json", jsonData, 0644)
		if err != nil {
			fmt.Printf("JSON 파일 저장 실패: %v\n", err)
			return
		}
		fmt.Println("✅ JSON 결과를 whisper_output.json 으로 저장 완료")

	} else {
		// ✅ JSON이 아닌 경우 (srt, vtt, text 등)
		ext := "." + responseFormat
		if responseFormat == "text" {
			ext = ".txt"
		}

		err := os.WriteFile("whisper_output"+ext, responseBody, 0644)
		if err != nil {
			fmt.Printf("파일 저장 실패: %v\n", err)
			return
		}
		fmt.Printf("✅ %s 결과를 whisper_output%s 으로 저장 완료\n", responseFormat, ext)
	}
}

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %s", err)
		return
	}

	// 전체 실행 시간 측정 시작
	totalStart := time.Now()

	samplePath := "./sample.mp4"
	extractStart := time.Now()
	audioPath, err := extractAudioToMP3(samplePath)
	if err != nil {
		fmt.Printf("Error extracting audio to MP3: %s", err)
		return
	}
	fmt.Printf("result audioPath : %s\n", audioPath)
	extractDuration := time.Since(extractStart)
	fmt.Printf("⏱️  오디오 추출 시간: %v\n", extractDuration)
	fmt.Printf("📁 결과 오디오 파일: %s\n", audioPath)

	apiStart := time.Now()

	transcribeAudio(config, audioPath)

	apiDuration := time.Since(apiStart)
	fmt.Printf("⏱️  API 호출 시간: %v\n", apiDuration)

	// 전체 실행 시간 계산
	totalDuration := time.Since(totalStart)

	// time check
	fmt.Println("========================================")
	fmt.Printf("📊 Results\n")
	fmt.Println("========================================")
	fmt.Printf("오디오 추출:     %8v (%5.1f%%)\n", extractDuration, float64(extractDuration.Nanoseconds())/float64(totalDuration.Nanoseconds())*100)
	fmt.Printf("API 호출:       %8v (%5.1f%%)\n", apiDuration, float64(apiDuration.Nanoseconds())/float64(totalDuration.Nanoseconds())*100)
	fmt.Println("========================================")
	fmt.Printf("전체 실행 시간:   %8v (100.0%%)\n", totalDuration)
}
