package main

import (
	"fmt"
	"strings"
)

func main() {
	sample := ""
	result := hasRepetitivePattern(sample, 0.5)
	fmt.Println(result)
}

// 반복패턴 체크
func hasRepetitivePattern(text string, minRatio float64) bool {
	words := strings.Fields(strings.ToLower(text))
	fmt.Printf("words: %v\n", words)

	if len(words) < 10 {
		return false
	}

	wordCount := make(map[string]int, len(words))
	for _, word := range words {
		if len(word) > 2 {
			wordCount[word]++
		}
	}

	maxCount := 0
	for _, count := range wordCount {
		if count > maxCount {
			maxCount = count
		}
	}

	ratio := float64(maxCount) / float64(len(words))
	if ratio >= minRatio {
		return true
	}

	if hasRepeatingNgrams(words, 2, 5, minRatio) {
		return true
	}

	return false
}

// hasRepeatingNgrams : N-gram 반복패턴 체크
func hasRepeatingNgrams(words []string, ngramSize, minRepeat int, minRatio float64) bool {
	if len(words) < ngramSize*minRepeat {
		return false
	}

	ngramCount := make(map[string]int)

	for i := 0; i <= len(words)-ngramSize; i++ {
		ngram := strings.Join(words[i:i+ngramSize], " ")
		ngramCount[ngram]++
	}

	totalNgrams := len(words) - ngramSize + 1
	for _, count := range ngramCount {
		if count >= minRepeat {
			ratio := float64(count) / float64(totalNgrams)
			if ratio >= minRatio {
				return true
			}
		}
	}

	return false
}
