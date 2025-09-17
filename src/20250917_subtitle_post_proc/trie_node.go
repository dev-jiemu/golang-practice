package main

import (
	"regexp"
	"strings"
)

// TrieNode : 트라이 노드 구조체
type TrieNode struct {
	Children        map[rune]*TrieNode
	IsEndOfWord     bool
	ReplacementWord string
}

// WordReplacementTrie : 트라이 구조체
type WordReplacementTrie struct {
	Root *TrieNode
}

func NewTrieNode() *TrieNode {
	return &TrieNode{
		Children:        make(map[rune]*TrieNode),
		IsEndOfWord:     false,
		ReplacementWord: "",
	}
}

func NewWordReplacementTrie() *WordReplacementTrie {
	return &WordReplacementTrie{
		Root: NewTrieNode(),
	}
}

// BuildTrie :JSON 데이터로 트라이 구조 초기화
func (trie *WordReplacementTrie) BuildTrie(jsonData *Dict) {
	for _, entry := range jsonData.Entries {
		targetWord := entry.Word

		for _, pattern := range entry.Patterns {
			trie.Insert(strings.ToLower(pattern.Value), targetWord)
		}
	}
}

// Insert : 패턴을 트라이에 삽입
func (trie *WordReplacementTrie) Insert(pattern, replacementWord string) {
	current := trie.Root

	for _, char := range pattern {
		if current.Children[char] == nil {
			current.Children[char] = NewTrieNode()
		}
		current = current.Children[char]
	}

	current.IsEndOfWord = true
	current.ReplacementWord = replacementWord
}

// Search : 주어진 단어가 트라이에 있는지 확인하고 치환 단어 반환
func (trie *WordReplacementTrie) Search(word string) (string, bool) {
	current := trie.Root
	lowerWord := strings.ToLower(word)

	for _, char := range lowerWord {
		if current.Children[char] == nil {
			return "", false // 패턴이 존재하지 않음
		}
		current = current.Children[char]
	}

	if current.IsEndOfWord {
		return current.ReplacementWord, true
	}
	return "", false
}

// ReplaceWords : 텍스트에서 단어들을 치환
func (trie *WordReplacementTrie) ReplaceWords(text string) string {
	wordRegex := regexp.MustCompile(`\w+`)

	result := wordRegex.ReplaceAllStringFunc(text, func(word string) string {
		if replacement, found := trie.Search(word); found {
			return replacement
		}
		return word
	})

	return result
}
