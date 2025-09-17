package main

import "fmt"

type Dict struct {
	Version string  `json:"version"`
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Word     string    `json:"word"`
	Patterns []Pattern `json:"patterns"`
}

type Pattern struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Flags string `json:"flags,omitempty"` // e.g., "i"
}

const (
	PatternTypeLiteral = "literal"
	PatternTypeRegex   = "regex"
)

func (v *Pattern) RequiredType(patternType string) error {
	if patternType != PatternTypeRegex && patternType != PatternTypeLiteral {
		return fmt.Errorf("required type %s not supported", patternType)
	}

	return nil
}
