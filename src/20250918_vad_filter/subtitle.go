package main

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/streamer45/silero-vad-go/speech"
)

type SubtitleSegment struct {
	Idx                     int              `json:"idx"`
	StartTime               float64          `json:"start_time"`
	EndTime                 float64          `json:"end_time"`
	Sentence                string           `json:"sentence"`
	SentenceConfidenceScore float64          `json:"sentence_confidence_score"`
	LLMCorrectSentence      string           `json:"llm_correct_sentence"`
	SentenceFrames          []SentenceFrames `json:"sentence_frames"`
}

type SentenceFrames struct {
	WordIdx         int     `json:"word_idx"`
	Word            string  `json:"word"`
	WordStartTime   float64 `json:"word_start_time"`
	WordEndTime     float64 `json:"word_end_time"`
	ConfidenceScore float64 `json:"confidence_score"`
}

func SortSubtitleSegment(segments []SubtitleSegment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime < segments[j].StartTime
	})
}

const THRESHOLD = 0.5 // 오차범위

// convertWhisperResponse : OpenAI 응답 데이터를 json, srt 파일로 만들기 위한 struct 생성 (json format)
func convertWhisperResponse(response WhisperResponse, filterSpeech []speech.Segment) []SubtitleSegment {
	convertedSegments := make([]SubtitleSegment, 0, len(response.Segments))
	validSegments := make([]SubtitleSegment, 0, len(response.Segments))

	wordTotalLength := len(response.Words)
	wordCursor := 0 // 과도한 탐색 방지를 위해 직전에 탐색한 인덱스 값 담아두는 용도

	// 권장: 30~80ms 사이에서 샘플 보며 조정 (챗지피티가 권장해줌 ㅇㅂㅇ)
	const tolerance = 0.05

	// 1. segment data to subtitle segment
	// LLMCorrectSentence => 이걸 whisper 가 안주는것 같음...
	for _, seg := range response.Segments {

		// Text 가 없으면 건너뜀
		if len(seg.Text) == 0 {
			continue
		}

		// subtitle object create
		subtitle := SubtitleSegment{
			Idx:                     seg.ID,
			StartTime:               roundSeconds(seg.Start),
			EndTime:                 roundSeconds(seg.End),
			Sentence:                normalizeWhitespace(seg.Text),
			SentenceConfidenceScore: seg.AvgLogProb,
		}

		/*
			2. sentence frame data insert
			* sentenceFrames 필드 관련 인지해야 할 내용..;ㅅ;
			OpenAi Whisper 의 경우 segment, word 데이터를 각각 다르게 줌;;; 그것도 segment내에 해당하는 모든 단어 다 합쳐서 뭉텅이로줌;;
			일단 segment 데이터 먼저 변환한 후 해당 데이터의 sentence 데이터 확인해서 word array 를 다시 조합하는 방법으로 풀어봄
			정확한 방법인진 모르겠음
		*/

		// words array index search
		startIdx := wordCursor
		for startIdx < wordTotalLength && (response.Words[startIdx].End+tolerance) < seg.Start {
			startIdx++
		}

		endIdx := startIdx
		for endIdx < wordTotalLength && (response.Words[endIdx].Start-tolerance) <= seg.End {
			endIdx++
		}

		// [startIdx, endIdx)
		frames := make([]SentenceFrames, 0, endIdx-startIdx)
		frameIdx := 0

		for k := startIdx; k < endIdx; k++ {
			word := response.Words[k]

			// 혹시모르니 가드
			if (word.End+tolerance) < seg.Start || (word.Start-tolerance) > seg.End {
				continue
			}

			frames = append(frames, SentenceFrames{
				WordIdx:       frameIdx,
				Word:          normalizeWhitespace(word.Word), // 앞뒤/다중 공백 정리
				WordStartTime: roundSeconds(word.Start),
				WordEndTime:   roundSeconds(word.End),
				// Whisper는 confidence_score 안줌...ㅠㅠ
			})
			frameIdx++
		}

		subtitle.SentenceFrames = frames
		convertedSegments = append(convertedSegments, subtitle)

		// next search index update
		wordCursor = endIdx
	}

	if len(filterSpeech) == 0 {
		return convertedSegments
	}

	/**
	[KOL-7916] 음성구간이 아님에도 자막데이터가 만들어질 경우, 자막 생성에서 제외처리
	Role: 임계값 0.5초, 부분겹침 구간은 인정해주고 완전히 겹치지 않는 데이터만 제거
	*/

	// 이진탐색 처리를 위해 데이터 정렬 (안해도 될것 같긴 한데 혹시모르니까)
	SortSubtitleSegment(convertedSegments)
	if len(filterSpeech) > 0 {
		sortSpeechSegments(filterSpeech)
	}

	for _, seg := range convertedSegments {
		startIdx := findContainingSegment(filterSpeech, seg.StartTime)
		endIdx := findContainingSegment(filterSpeech, seg.EndTime)

		// 둘중에 하나라도 index 를 찾았으면 범위 안에 들어간다고 간주함
		if startIdx != -1 || endIdx != -1 {
			validSegments = append(validSegments, seg)
		}
	}

	return validSegments
}

// convertSegmentToSrtFormat : segments data SRT 포맷으로 변경
func convertSegmentToSrtFormat(segments []SubtitleSegment) []byte {
	var buffer bytes.Buffer

	optimizedTimes := make([][2]int, len(segments)) // [startMs, endMs]

	for idx, current := range segments {
		startTime := int(math.Round(current.StartTime * 1000))
		endTime := int(math.Round(current.EndTime * 1000))

		// 갭처리 로직
		// whisper 로 json, srt 데이터 각각 추출해봤을 때 timestamp 값의 차이가 어느정도 보임
		// json 데이터는 말하는 그 순간의 timeline 정보만 주는것 같고, srt 파일은 다음 말하기까지의 기다리는 시간도 포함해서 설정되는 듯함
		if idx > 0 {
			gap := startTime - optimizedTimes[idx-1][1]

			if gap < 40 {
				startTime = optimizedTimes[idx-1][1]
			} else if gap < 150 {
				midPoint := optimizedTimes[idx-1][1] + gap/2
				optimizedTimes[idx-1][1] = midPoint
				startTime = midPoint + 1
			}
		}

		// 다음 segment 시작 시간보다 크면 안되니까 조정함
		if idx < len(segments)-1 {
			nextStart := int(segments[idx+1].StartTime * 1000)
			if endTime > nextStart {
				endTime = nextStart - 1
			}
		}

		duration := endTime - startTime
		minDuration := max(500, len(current.Sentence)*50) // Text -> Sentence
		maxDuration := 5000

		if duration < minDuration {
			endTime = startTime + minDuration

			if idx < len(segments)-1 {
				nextStart := int(segments[idx+1].StartTime * 1000)
				if endTime > nextStart {
					startTime = nextStart - minDuration
					endTime = nextStart - 1
				}
			} else if duration > maxDuration {
				endTime = startTime + maxDuration
			}

			if startTime < 0 {
				startTime = 0
			}
		}

		optimizedTimes[idx] = [2]int{startTime, endTime}
	}

	for idx, current := range segments {
		startTime := optimizedTimes[idx][0]
		endTime := optimizedTimes[idx][1]

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

func findContainingSegment(speech []speech.Segment, time float64) int {
	if len(speech) == 0 {
		return -1 // 전체가 무음구간일 경우 그냥 종료처리
	}

	left, right := 0, len(speech)-1

	for left <= right {
		mid := (left + right) / 2
		segment := speech[mid]

		// 오차를 고려한 범위 체크
		adjustedStart := segment.SpeechStartAt - THRESHOLD
		adjustedEnd := segment.SpeechEndAt + THRESHOLD

		if time >= adjustedStart && time <= adjustedEnd {
			return mid
		} else if time < adjustedStart {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return -1 // 못찾음
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

// sortSpeechSegments : start_at 오름차순으로 객체 정렬
func sortSpeechSegments(segments []speech.Segment) {
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].SpeechStartAt < segments[j].SpeechStartAt
	})
}
