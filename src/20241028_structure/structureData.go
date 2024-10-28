package main

import (
	"fmt"
	"strconv"
)

type Item struct {
	Type     string `json:"type"`
	Seqno    int    `json:"seqno"`
	Children []Item `json:"children,omitempty"`
}

func structureData(data []map[string]string) []Item {
	result := make([]Item, 0, len(data))
	stack := make([]*[]Item, 1, len(data))
	stack[0] = &result
	prevDepth := 0

	for _, item := range data {
		sequence := item["sequence"]
		depth := len(sequence) / 2

		for depth < prevDepth {
			stack = stack[:len(stack)-1]
			prevDepth--
		}

		fmt.Printf("len(sequence)-2 : %d, sequence Atoi : %s\n", len(sequence)-2, sequence[len(sequence)-2:])
		seqno, _ := strconv.Atoi(sequence[len(sequence)-2:])
		current := Item{
			Type:  fmt.Sprintf("level_%d", depth),
			Seqno: seqno,
		}

		if depth > prevDepth {
			if len(*stack[len(stack)-1]) > 0 {
				lastIndex := len(*stack[len(stack)-1]) - 1
				(*stack[len(stack)-1])[lastIndex].Children = append((*stack[len(stack)-1])[lastIndex].Children, current)
				stack = append(stack, &(*stack[len(stack)-1])[lastIndex].Children)
			} else {
				*stack[len(stack)-1] = append(*stack[len(stack)-1], current)
				stack = append(stack, &(*stack[len(stack)-1])[len(*stack[len(stack)-1])-1].Children)
			}
			prevDepth++
		} else {
			*stack[len(stack)-1] = append(*stack[len(stack)-1], current)
		}
	}

	return result
}

func main() {
	data := []map[string]string{
		{"sequence": "01"},
		{"sequence": "0101"},
		{"sequence": "010101"},
		{"sequence": "010102"},
		{"sequence": "0102"},
		{"sequence": "010201"},
		{"sequence": "010202"},
		{"sequence": "02"},
		{"sequence": "0201"},
		{"sequence": "0202"},
		{"sequence": "020201"},
	}

	structuredData := structureData(data)
	fmt.Printf("%+v\n", structuredData)
}
