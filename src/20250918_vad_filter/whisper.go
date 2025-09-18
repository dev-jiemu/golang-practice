package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
)

type WhisperResponse struct {
	Task     string    `json:"task"`
	Language string    `json:"language"`
	Duration float64   `json:"duration"`
	Text     string    `json:"text"`
	Words    []Word    `json:"words"`
	Segments []Segment `json:"segments"`
}

type Word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Segment struct {
	ID               int     `json:"id"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	Words            []Word  `json:"words,omitempty"` // 세그먼트 안에 단어 단위가 들어오는 경우
}

func TranscribeAudio(config *Config, filePath string) {
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		fmt.Printf("파일을 찾을 수 없습니다: %s\n", filePath)
		return
	}
	if err != nil {
		fmt.Printf("파일 정보를 가져올 수 없습니다: %v\n", err)
		return
	}

	// 파일 크기 체크 (25MB = 25 * 1024 * 1024 bytes)
	const maxFileSize = 25 * 1024 * 1024
	fileSize := fileInfo.Size()
	fileSizeMB := float64(fileSize) / (1024 * 1024)

	fmt.Printf("파일 크기: %.2f MB\n", fileSizeMB)

	if fileSize > maxFileSize {
		fmt.Printf("파일 크기가 너무 큽니다 (%.2f MB). OpenAI Whisper API는 최대 25MB까지만 지원합니다. 파일을 압축하거나 분할해주세요\n", fileSizeMB)
		return
	}

	// 파일 열기
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("파일을 열 수 없습니다: %v\n", err)
		return
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(filePath)))
	header.Set("Content-Type", "audio/webm")
	part, err := writer.CreatePart(header)
	if err != nil {
		fmt.Errorf("create part: %w", err)
		return
	}

	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Printf("파일 복사 실패: %v", err)
		return
	}

	_ = writer.WriteField("model", "whisper-1")
	_ = writer.WriteField("language", "en")
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("timestamp_granularities[]", "word")
	_ = writer.WriteField("timestamp_granularities[]", "segment")

	err = writer.Close()
	if err != nil {
		fmt.Printf("multipart writer close fail : %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &body)
	if err != nil {
		fmt.Printf("HTTP 요청 생성 실패: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+config.OpenAIKey)

	fmt.Printf("Content-Type: %s\n", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("HTTP 요청 실행 실패: %v\n", err)
	}
	defer resp.Body.Close()

	fmt.Printf("result status code: %d\n", resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("응답 읽기 실패: %v\n", err)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API 오류 (상태코드: %d): %s\n", resp.StatusCode, string(responseBody))
	}

	var whisperResponse WhisperResponse
	err = json.Unmarshal(responseBody, &whisperResponse)
	if err != nil {
		fmt.Printf("JSON 파싱 실패: %v\n", err)
	}

	fmt.Printf("result : [%+v]\n", whisperResponse)

	jsonData, err := json.MarshalIndent(whisperResponse, "", "  ")
	if err != nil {
		fmt.Printf("JSON 마샬링 실패: %v\n", err)
		return
	}

	filename := "whisper_output_prompt.json"
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Printf("파일 저장 실패: %v\n", err)
		return
	}
}
