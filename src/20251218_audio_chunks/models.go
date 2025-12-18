package main

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/streamer45/silero-vad-go/speech"
)

type Job struct {
	RId               string
	OriginalAudioPath string
	WavAudioPath      string // mp4 -> mp3
	FilteredAudioPath string // 무음구간 필터 적용파일 경로
	AudioPath         string // .webm
	ChunkPath         string
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
	MinDurationSec float64
	MaxDurationSec float64
	OverlapSec     float64
}

type SubtitleSegment struct {
	Idx                     int              `json:"idx"`
	StartTime               float64          `json:"start_time"`
	EndTime                 float64          `json:"end_time"`
	Sentence                string           `json:"sentence"`
	SentenceConfidenceScore float64          `json:"sentence_confidence_score"`
	LLMCorrectSentence      string           `json:"llm_correct_sentence"`
	SentenceFrames          []SentenceFrames `json:"sentence_frames"`
	NoSpeechProb            float64          `json:"no_speech_prob,omitempty"`
	CompressionRatio        float64          `json:"compression_ratio,omitempty"`
}

type SentenceFrames struct {
	WordIdx         int     `json:"word_idx"`
	Word            string  `json:"word"`
	WordStartTime   float64 `json:"word_start_time"`
	WordEndTime     float64 `json:"word_end_time"`
	ConfidenceScore float64 `json:"confidence_score"`
}

const (
	MaxChars    = 84  // 2줄 기준
	MaxDuration = 4.0 // 4초
	MinDuration = 1.5 // 1.5초
)

func SortSubtitleSegment(segments []SubtitleSegment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime < segments[j].StartTime
	})
}

func SplitLongSegments(segments []SubtitleSegment) []SubtitleSegment {
	newSegments := make([]SubtitleSegment, 0, len(segments)*2) // 일단 2배로 잡고 봄
	globalIdx := segments[0].Idx

	for _, seg := range segments {
		// 분할이 필요한지 체크
		if shouldSplit(seg) {
			splits := splitSegment(&seg, globalIdx)
			newSegments = append(newSegments, splits...)
			globalIdx += len(splits)
		} else {
			seg.Idx = globalIdx
			newSegments = append(newSegments, seg)
			globalIdx++
		}
	}

	return newSegments
}

func shouldSplit(seg SubtitleSegment) bool {
	duration := seg.EndTime - seg.StartTime
	return len(seg.Sentence) > MaxChars || duration > MaxDuration
}

func splitSegment(segment *SubtitleSegment, startIdx int) []SubtitleSegment {
	splits := make([]SubtitleSegment, 0, len(segment.SentenceFrames)+1) // SentenceFrames size 보단 적게 생성될거니까

	// sentence 가 "" 상황을 체크해야하는게 맞는데, 이미 convert 단계에서 빈객체일 경우 생성하지 않아서 굳이 체크 안함
	currentSplit := SubtitleSegment{
		Idx:                     startIdx,
		StartTime:               segment.SentenceFrames[0].WordStartTime,
		SentenceConfidenceScore: segment.SentenceConfidenceScore,
		SentenceFrames:          []SentenceFrames{},
	}

	currentText := ""
	splitIdx := 0
	for idx, frame := range segment.SentenceFrames {
		currentSplit.SentenceFrames = append(currentSplit.SentenceFrames, frame)
		currentText += frame.Word + " "

		currentDuration := frame.WordEndTime - currentSplit.StartTime
		shouldBreak := false

		word := strings.TrimSpace(frame.Word)

		// 1. 끝문장임 + 최소 시간 보다 넘었음
		if isEndSentence(word) && currentDuration >= MinDuration {
			shouldBreak = true
		}

		// 2. 글자 수 초과
		if len(currentText) >= MaxChars {
			shouldBreak = true
		}

		// 3. 시간 초과
		if currentDuration >= MaxDuration {
			shouldBreak = true
		}

		// 마지막 단어 또는 분할일때
		if shouldBreak || idx == len(segment.SentenceFrames)-1 {
			currentSplit.EndTime = frame.WordEndTime
			currentSplit.Sentence = strings.TrimSpace(currentText)
			splits = append(splits, currentSplit)

			// 다음 split 준비
			if idx < len(segment.SentenceFrames)-1 {
				splitIdx++
				currentSplit = SubtitleSegment{
					Idx:                     startIdx + splitIdx,
					StartTime:               segment.SentenceFrames[idx+1].WordStartTime,
					SentenceConfidenceScore: segment.SentenceConfidenceScore,
					SentenceFrames:          []SentenceFrames{},
				}
				currentText = ""
			}
		}
	}

	return splits
}

func isEndSentence(word string) bool {
	if strings.HasSuffix(word, ".") || strings.HasSuffix(word, ",") || strings.HasSuffix(word, "!") || strings.HasSuffix(word, "?") {
		return true
	}

	return false
}

var multiSpace = regexp.MustCompile(`\s+`)

// normalizeWhitespace : 앞뒤 공백 제거 + 연속 공백을 한 칸으로
func normalizeWhitespace(s string) string {
	s = strings.TrimSpace(s)
	s = multiSpace.ReplaceAllString(s, " ")
	return s
}

// 반올림 헬퍼: 2자리 소수로
func roundSeconds(x float64) float64 {
	pow := math.Pow(10, 2) // 100
	return math.Round(x*pow) / pow
}
