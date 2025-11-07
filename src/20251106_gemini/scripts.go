package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("사용법: %s <입력파일.확장자명> <출력파일.확장자명>", os.Args[0])
	}

	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("파일 읽기 실패: %v\n", err)
		return
	}

	ctx := context.Background()

	const apiKey = ""
	const modelName = "gemini-2.5-flash"

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatal(err)
	}

	instruction := `
	나는 B2B 비디오 스트리밍 플랫폼 회사에서 Product를 담당하고 있습니다. 주요 고객사는 한국의 교육 콘텐츠 서비스사로, 중고등학생이나 성인들의 교육 영상을 서비스하는 회사들입니다. 
	우리는 고객사들이 더 많은 수익을 얻을 수 있게 AI 자막이나 AI 요약 등 다양한 서비스들을 개발하고 있습니다. 인하우스 서비스와 달리 여러 고객사들의 니즈와 다양한 상황을 고려해 서비스를 기획해야 합니다.
	
	1. 작업 목표: Canary 자막 파일들의 텍스트를 교정합니다.
	2. 교정 원칙:
	글자만 수정: 잘못된 단어, 고유명사만 수정합니다. (1단계 원칙)
	추가 규칙: Culture Wave는 Kulture Wave로 변경합니다.
	시간/단위는 원본 유지: Canary 원본의 타임스탬프와 자막 분할 단위는 절대 변경하지 않습니다.
	`

	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: instruction},
				{Text: string(content)},
			},
		},
	}

	// token check
	info, _ := client.Models.CountTokens(ctx, modelName, contents, nil)
	fmt.Println("input tokens:", info.TotalTokens)

	result, err := client.Models.GenerateContent(ctx, modelName, contents, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("input token: %d, output token: %d\n", result.UsageMetadata.PromptTokenCount, result.UsageMetadata.CandidatesTokenCount)

	clean := extractSRT(result.Text())
	if err = os.WriteFile(os.Args[2], []byte(clean), 0644); err != nil {
		log.Fatalf("write file fail : %v", err)
	}

}

func extractSRT(raw string) string {
	s := strings.TrimSpace(raw)

	// 1) 코드펜스 내부만 추출 (```srt ... ``` 등 언어 태그 유무 모두 지원)
	codeFence := regexp.MustCompile("(?s)```[^\\n]*\\n(.*?)\\n?```")
	if m := codeFence.FindStringSubmatch(s); m != nil {
		s = m[1]
	}

	// 2) 맨 앞 라인이 "SRT" 라벨이면 제거 (대소문자 무시)
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && strings.EqualFold(strings.TrimSpace(lines[0]), "SRT") {
		lines = lines[1:]
		s = strings.Join(lines, "\n")
	}

	// 3) 개행 정규화 & 양끝 공백 정리
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	return s
}
