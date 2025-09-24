package main

import "fmt"

/*
*
2025.09.24 segment 정렬 샘플코드 (binary search)
- 오차범위 0.5s
*/

const THRESHOLD = 0.5 // 오차범위

func main() {
	speechSegments, whisperSegments := getTestData()

	// 시작 시간 기준으로 정렬 (안해도 될것 같긴한데 혹시몰라서 ㅇㅂㅇ...)
	SortSpeechSegments(speechSegments)
	SortWhisperSegments(whisperSegments)

	filteredSegments := FilterWhisperSegments(speechSegments, whisperSegments)

	// 결과 출력
	printResults(whisperSegments, filteredSegments)

}

func FilterWhisperSegments(speech []SpeechSegment, whisper []WhisperSegment) []WhisperSegment {
	filtered := make([]WhisperSegment, 0, len(whisper))

	for _, segment := range whisper {
		// 시간 포함여부 확인
		startNotContained := findContainingSegment(speech, segment.Start)
		endNotContained := findContainingSegment(speech, segment.End)

		// 둘다 포함이 안되는것만 제거할거임 (-1 은 못찾은거임)
		if startNotContained != -1 || endNotContained != -1 {
			filtered = append(filtered, segment)
		}
	}

	return filtered
}

func findContainingSegment(speech []SpeechSegment, time float64) int {
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

	return -1
}

// 결과 출력 함수
func printResults(original, filtered []WhisperSegment) {
	fmt.Println("=== 원본 WhisperSegment ===")
	for _, seg := range original {
		fmt.Printf("Index: %d, Start: %.3f, End: %.3f, Text: %s\n",
			seg.Id, seg.Start, seg.End, seg.Text)
	}

	fmt.Println("\n=== 필터링된 WhisperSegment ===")
	for _, seg := range filtered {
		fmt.Printf("Index: %d, Start: %.3f, End: %.3f, Text: %s\n",
			seg.Id, seg.Start, seg.End, seg.Text)
	}

	fmt.Printf("\n원본: %d개 -> 필터링 후: %d개 (제거된 개수: %d개)\n",
		len(original), len(filtered), len(original)-len(filtered))
}
