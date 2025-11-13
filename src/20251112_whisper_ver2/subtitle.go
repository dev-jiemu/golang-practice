package main

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

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

// ConvertSegmentToSrtFormat : segments data SRT 포맷으로 변경
// 2025.11.07 Canary 호출시 자막 타임라인이 겹치는 이슈가 있어서, 그냥 원 데이터 쓰는걸로 변경
func ConvertSegmentToSrtFormat(segments []SubtitleSegment) []byte {
	var buffer bytes.Buffer

	for idx, current := range segments {
		startTime := int(math.Round(current.StartTime * 1000))
		endTime := int(math.Round(current.EndTime * 1000))

		// 다음 segment 시작 시간보다 크면 안되니까 조정함
		if idx < len(segments)-1 {
			nextStart := int(math.Round(segments[idx+1].StartTime * 1000))
			if endTime > nextStart {
				endTime = nextStart - 1
			}
		}

		buffer.WriteString(fmt.Sprintf("%d\n", idx+1))
		buffer.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTime(startTime), formatSRTTime(endTime)))
		buffer.WriteString(fmt.Sprintf("%s\n\n", strings.TrimSpace(current.Sentence)))
	}

	// 마지막 개행 제거함
	result := buffer.Bytes()
	if len(result) > 1 && result[len(result)-1] == '\n' {
		return result[:len(result)-1]
	}

	return result
}

// formatSRTTime : timestamp format 변경 (ex. 20.15 -> 00:00:20,150)
// 2025.08.22 float -> int 변경 (roundSeconds 메서드로 인해 2자리 반올림 처리 된 상태로 작업이 진행됨)
func formatSRTTime(ms int) string {
	duration := time.Duration(ms) * time.Millisecond

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	milliseconds := ms % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
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
