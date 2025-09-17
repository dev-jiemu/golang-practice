package main

import (
	"fmt"
)

type Segment struct {
	Text string `json:"text"`
	Idx  int    `json:"idx"`
}

func ProcessSegments(segments []Segment, trie *WordReplacementTrie) []Segment {
	processedSegments := make([]Segment, len(segments))

	for i, segment := range segments {
		processedSegments[i] = Segment{
			Idx:  segment.Idx,
			Text: trie.ReplaceWords(segment.Text), // replace words
		}
	}

	return processedSegments
}

func main() {
	jsonData := &Dict{
		Version: "2025-09-17 14:30:00",
		Entries: []Entry{
			{
				Word: "hello",
				Patterns: []Pattern{
					{Value: "hEllo"},
					{Value: "HELLO"},
					{Value: "Hello"},
					{Value: "helo"},
				},
			},
			{
				Word: "world",
				Patterns: []Pattern{
					{Value: "World"},
					{Value: "WORLD"},
					{Value: "wrld"},
				},
			},
			{
				Word: "Go",
				Patterns: []Pattern{
					{Value: "golang"},
					{Value: "GO"},
					{Value: "go"},
					{Value: "Golang"},
				},
			},
		},
	}

	// 트라이 생성 및 초기화
	trie := NewWordReplacementTrie()
	trie.BuildTrie(jsonData)

	// 테스트용 세그먼트 데이터
	segments := []Segment{
		{Idx: 1, Text: "hEllo, World! This is a test."},
		{Idx: 2, Text: "I love golang and GO programming."},
		{Idx: 3, Text: "HELLO wrld, how are you?"},
	}

	processedSegments := ProcessSegments(segments, trie)

	fmt.Println("Original segments:")
	for _, segment := range segments {
		fmt.Printf("Idx: %d, Text: %s\n", segment.Idx, segment.Text)
	}

	fmt.Println("========================")

	fmt.Println("Processed segments:")
	for _, segment := range processedSegments {
		fmt.Printf("Idx: %d, Text: %s\n", segment.Idx, segment.Text)
	}
}
