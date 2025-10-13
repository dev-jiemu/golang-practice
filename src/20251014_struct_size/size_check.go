package main

import (
	"fmt"
	"time"
	"unsafe"
)

type Job struct {
	Monitoring *MonitoringData // 내부 로깅용 struct
	SrcPath    string          `json:"path"`
	Duration   int             `json:"duration"`   // 과금용
	ContentId  string          `json:"content_id"` // 과금용 (2025.09.25)
	JobId      int64           `json:"job_id"`     // 과금용 (2025.09.25)
	Start      string          `json:"start"`      // Job 최초 시작 시점
	End        string          `json:"end"`        // Job process 종료 시점
}

type MonitoringData struct {
	WhisperResponseCode    int             `json:"whisper_response_code"`
	WhisperResponseBodyLen int             `json:"whisper_response_body_length"`
	StorageUploadTime      string          `json:"storage_upload_time"` // timestamp
	SegmentCount           int             `json:"segment_count"`
	ProcessMetrics         *ProcessMetrics `json:"process_metrics"`
	SegmentFilter          *SegmentFilter  `json:"segment_filter"`
}

type ProcessMetrics struct {
	StartTime time.Time // 최초 메세지 수신 시간기준

	TotalDurationMs  int64 `json:"total_duration_ms"`
	AudioToWavTimeMs int64 `json:"audio_to_wav_time_ms"`
	WavToWebmTimeMs  int64 `json:"wav_to_webm_time_ms"`
	WhisperAPITimeMs int64 `json:"whisper_api_time_ms"`
}

type SegmentFilter struct {
	SilencedDurationMs float64 `json:"silenced_duration_ms"`
	SpeechDurationMs   float64 `json:"speech_duration_ms"`
}

func main() {
	var job Job
	fmt.Printf("Job struct 크기: %d bytes\n", unsafe.Sizeof(job))
}
