package main

import "github.com/streamer45/silero-vad-go/speech"

type Job struct {
	RId               string
	OriginalAudioPath string
	WavAudioPath      string // mp4 -> mp3
	FilteredAudioPath string // 무음구간 필터 적용파일 경로
	AudioPath         string // .webm
}

// WhisperResponse : verbose_json 일 경우 파싱할 객체
type WhisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Text     string           `json:"text"` // 이거 호출해보니 앞에 공백 붙어서 옴
	Words    []WhisperWord    `json:"words"`
	Segments []WhisperSegment `json:"segments"`
}

type WhisperWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type WhisperSegment struct {
	ID               int           `json:"id"`
	Seek             int           `json:"seek"` // 세그먼트 찾을때 오프셋 값
	Start            float64       `json:"start"`
	End              float64       `json:"end"`
	Text             string        `json:"text"`
	AvgLogProb       float64       `json:"avg_logprob"`       // 평균 로그 확률값. -1보다 낮으면 확률 계산에 실패했다고 간주함
	CompressionRatio float64       `json:"compression_ratio"` // 세그먼트 압축 비율. 2.4보다 크면 압축에 실패했다고 간주함
	NoSpeechProb     float64       `json:"no_speech_prob"`    // 세그먼트 안에 말소리가 없을 확률
	Temperature      float64       `json:"temperature"`       // 샘플링 온도(낮을수록 보수적인 결과)
	Words            []WhisperWord `json:"words,omitempty"`   // 세그먼트 안에 단어 단위가 들어오는 경우
}

// VADSidecar : 필터 관련 활용값이 필요하다면
type VADSidecar struct {
	SampleRate    int       `json:"sample_rate"`
	HopSec        float64   `json:"hop_sec"`         // e.g. 0.01
	NoiseFloorRMS float64   `json:"noise_floor_rms"` // 파일 레벨 바닥
	RMSEnvelope   []float64 `json:"rms_envelope"`    // 10ms frame RMS
}

// PadMetrics : VAD Filter 적용 전 speech pad, min silence 결정을 위한 객체
type PadMetrics struct {
	SNRdB                 float64
	AvgSilenceSec         float64
	ShortSilenceRatio     float64
	NoiseFloorRMS         float64
	SpeechLevelRMS        float64
	RawPadMs              float64
	FinalPadMs            int
	SuggestedMinSilenceMs int
}

type AudioChunk struct {
	StartSec   float64
	EndSec     float64
	OverlapSec float64

	// 청크에 속한 VAD 세그먼트들
	VADSegments []speech.Segment

	// 디버깅용
	Index    int
	Duration float64
}

type ChunkingConfig struct {
	TargetDurationSec float64 // 60.0 (1분)
	MinDurationSec    float64 // 10.0 (10초)
	MaxDurationSec    float64 // 120.0 (2분)
	OverlapSec        float64 // 1.5 (1.5초)
}
