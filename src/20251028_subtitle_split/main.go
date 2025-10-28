package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	// 입력 파일과 출력 파일 경로
	inputFile := "./compare/arirang_1_filtered_100.srt"
	outputFile := "./compare/arirang_1_filtered_100.txt"

	// SRT 파일 읽기
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("파일 읽기 실패: %v\n", err)
		return
	}

	// 텍스트만 추출
	textOnly := extractTextFromSRT(string(content))

	// 결과를 파일로 저장
	err = os.WriteFile(outputFile, []byte(textOnly), 0644)
	if err != nil {
		fmt.Printf("파일 저장 실패: %v\n", err)
		return
	}

	fmt.Printf("완료! %s 파일이 생성되었습니다.\n", outputFile)
}

func extractTextFromSRT(content string) string {
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))

	// 인덱스 번호를 매칭하는 정규식 (숫자만 있는 줄)
	indexRegex := regexp.MustCompile(`^\d+$`)

	// 타임라인을 매칭하는 정규식 (00:00:00,000 --> 00:00:00,000 형식)
	timelineRegex := regexp.MustCompile(`^\d{2}:\d{2}:\d{2},\d{3}\s*-->\s*\d{2}:\d{2}:\d{2},\d{3}`)

	lastWasText := false // 마지막이 텍스트였는지 추적

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 빈 줄 처리 - 자막 블록 구분용
		if line == "" {
			if lastWasText {
				result.WriteString("\n") // 블록 사이 구분을 위한 빈 줄 추가
				lastWasText = false
			}
			continue
		}

		// 인덱스 번호 줄은 건너뛰기
		if indexRegex.MatchString(line) {
			continue
		}

		// 타임라인 줄은 건너뛰기
		if timelineRegex.MatchString(line) {
			continue
		}

		// 자막 텍스트만 추가
		result.WriteString(line)
		result.WriteString("\n")
		lastWasText = true
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("스캔 중 오류: %v\n", err)
	}

	return result.String()
}
