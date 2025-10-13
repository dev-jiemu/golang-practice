package main

/*
// macOS용 (로컬 개발)
#cgo darwin CFLAGS: -I/usr/local/onnxruntime-osx-arm64-1.18.1/include
#cgo darwin LDFLAGS: -L/usr/local/onnxruntime-osx-arm64-1.18.1/lib -lonnxruntime -Wl,-rpath,/usr/local/onnxruntime-osx-arm64-1.18.1/lib
*/
import "C"

// CGO 지시어 설명:
// #cgo CFLAGS: C 컴파일러에게 전달할 플래그 (헤더 파일 경로 등)
// #cgo LDFLAGS: 링커에게 전달할 플래그 (라이브러리 경로, 링크할 라이브러리 등)
//
// 이렇게 설정하면:
// 기존: CGO_LDFLAGS="..." go run *.go
// 이제: go run *.go  <- 그냥 이렇게만 실행!

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

/**
wget https://github.com/snakers4/silero-vad/raw/master/src/silero_vad/data/silero_vad.onnx -O /usr/local/silero_vad.onnx
wget https://github.com/microsoft/onnxruntime/releases/download/v1.18.1/onnxruntime-linux-x64-1.18.1.tgz \
    && tar -xzf onnxruntime-linux-x64-1.18.1.tgz \
    && mv onnxruntime-linux-x64-1.18.1 /usr/local/onnxruntime \
    && rm onnxruntime-linux-x64-1.18.1.tgz

ENV LIBRARY_PATH="/usr/local/onnxruntime/lib:$LIBRARY_PATH"
ENV C_INCLUDE_PATH="/usr/local/onnxruntime/include:$C_INCLUDE_PATH"
ENV LD_LIBRARY_PATH="/usr/local/onnxruntime/lib:$LD_LIBRARY_PATH"
ENV LD_RUN_PATH="/usr/local/onnxruntime/lib:$LD_RUN_PATH"
*/

type Config struct {
	OpenAIKey string `json:"openai-key"`
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

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("사용법: %s <입력파일.mp4>", os.Args[0])
	}

	//config, err := loadConfig()
	//if err != nil {
	//	fmt.Printf("Error loading config: %s", err)
	//	return
	//}

	totalStart := time.Now()
	wavExtractStart := time.Now()
	wavAudioPath, err := ExtractAudioToWav(os.Args[1])

	if err != nil {
		log.Fatalf("Error extracting audio: %v", err)
	}

	fmt.Printf("result wav audio path : %s\n", wavAudioPath)
	wavExtractDuration := time.Since(wavExtractStart)
	fmt.Printf("⏱️ 오디오 추출 시간(mp4 to wav): %v\n", wavExtractDuration)
	fmt.Printf("📁 결과 오디오 파일: %s\n", wavAudioPath)

	filterStart := time.Now()

	_, resultFilterPath, err := VadFilter(wavAudioPath)
	filterDuration := time.Since(filterStart)
	fmt.Printf("⏱️ 무음구간 변환 시간: %v\n", filterDuration)
	fmt.Printf("📁 결과 오디오 파일: %s\n", resultFilterPath)

	extractStart := time.Now()
	audioPath, err := ExtractAudioToMP3(resultFilterPath)
	if err != nil {
		fmt.Printf("Error extracting audio to MP3: %s", err)
		return
	}
	fmt.Printf("result audioPath : %s\n", audioPath)
	extractDuration := time.Since(extractStart)
	fmt.Printf("⏱️  오디오 추출 시간: %v\n", extractDuration)
	fmt.Printf("📁 결과 오디오 파일: %s\n", audioPath)

	//apiStart := time.Now()
	//
	//TranscribeAudio(config, audioPath, filterSegments)
	////TranscribeAudio(config, audioPath, make([]speech.Segment, 0))
	//
	//apiDuration := time.Since(apiStart)
	//fmt.Printf("⏱️  API 호출 시간: %v\n", apiDuration)
	//
	// 전체 실행 시간 계산
	totalDuration := time.Since(totalStart)

	// time check
	fmt.Println("========================================")
	fmt.Printf("📊 Results\n")
	fmt.Println("========================================")
	fmt.Printf("오디오 추출(mp4 to wav):     %8v (%5.1f%%)\n", wavExtractDuration, float64(wavExtractDuration.Nanoseconds())/float64(totalDuration.Nanoseconds())*100)
	fmt.Printf("오디오 변환(wav to webm):     %8v (%5.1f%%)\n", extractDuration, float64(extractDuration.Nanoseconds())/float64(totalDuration.Nanoseconds())*100)
	//fmt.Printf("API 호출:       %8v (%5.1f%%)\n", apiDuration, float64(apiDuration.Nanoseconds())/float64(totalDuration.Nanoseconds())*100)
	fmt.Println("========================================")
	fmt.Printf("전체 실행 시간:   %8v (100.0%%)\n", totalDuration)
}
