package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

type WhisperSegment struct {
	ID               int     `json:"id"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	AvgLogProb       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

// 2-pass 처리
func generateSRTOptimized(segments []WhisperSegment) []byte {
	if len(segments) == 0 {
		return []byte{}
	}

	// 1단계: 타임스탬프 최적화 (메모리 효율적)
	optimizedTimes := make([][2]int, len(segments)) // [startMs, endMs]

	for i, current := range segments {
		startTime := int(math.Round(current.Start * 1000))
		endTime := int(math.Round(current.End * 1000))

		// 이전 세그먼트와의 갭 처리
		if i > 0 {
			gap := startTime - optimizedTimes[i-1][1]

			if gap < 100 {
				startTime = optimizedTimes[i-1][1]
			} else if gap < 500 {
				midPoint := optimizedTimes[i-1][1] + gap/2
				optimizedTimes[i-1][1] = midPoint // 이전 세그먼트 끝시간 수정
				startTime = midPoint + 1
			}
		}

		// 다음 세그먼트와의 겹침 방지
		if i < len(segments)-1 {
			nextStart := int(math.Round(segments[i+1].Start * 1000))
			if endTime > nextStart {
				endTime = nextStart - 1
			}
		}

		// 최소/최대 지속시간 보장
		duration := endTime - startTime
		minDuration := max(500, len(current.Text)*50)
		maxDuration := 5000

		if duration < minDuration {
			endTime = startTime + minDuration

			if i < len(segments)-1 {
				nextStart := int(math.Round(segments[i+1].Start * 1000))
				if endTime > nextStart {
					startTime = nextStart - minDuration
					endTime = nextStart - 1
				}
			}
		} else if duration > maxDuration {
			endTime = startTime + maxDuration
		}

		if startTime < 0 {
			startTime = 0
		}

		optimizedTimes[i] = [2]int{startTime, endTime}
	}

	// 2단계: SRT 형식으로 출력
	var buffer bytes.Buffer

	for i, segment := range segments {
		startTime := optimizedTimes[i][0]
		endTime := optimizedTimes[i][1]

		buffer.WriteString(fmt.Sprintf("%d\n", i+1))
		buffer.WriteString(fmt.Sprintf("%s --> %s\n", msToSRTTime(startTime), msToSRTTime(endTime)))
		buffer.WriteString(fmt.Sprintf("%s\n\n", strings.TrimSpace(segment.Text)))
	}

	// 마지막 개행 제거
	result := buffer.Bytes()
	if len(result) > 1 && result[len(result)-1] == '\n' {
		return result[:len(result)-1]
	}

	return result
}

func msToSRTTime(ms int) string {
	duration := time.Duration(ms) * time.Millisecond

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	milliseconds := ms % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// 사용 예시
func main() {
	segments := []WhisperSegment{
		{
			ID:    0,
			Start: 11.5,
			End:   15.819999694824219,
			Text:  " Okay, well, it's three minutes past the hour, so we'll start officially.",
		},
		{
			ID:    1,
			Start: 16.34000015258789,
			End:   16.959999084472656,
			Text:  " Welcome everyone.",
		},
		{
			ID:    2,
			Start: 17.219999313354492,
			End:   22.920000076293945,
			Text:  " My name is Duja Dvizna, I'm the Director of Policy and Open Culture at Creative Commons,",
		},
	}

	// 최적화된 방식 사용
	srtBytes := generateSRTOptimized(segments)
	fmt.Printf("Generated SRT (%d bytes):\n%s\n", len(srtBytes), string(srtBytes))

	err := os.WriteFile("output.srt", srtBytes, 0644)
	if err != nil {
		_ = fmt.Errorf("Error writing SRT: %v", err)
	}
}
