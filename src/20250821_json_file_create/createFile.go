package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Person struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func main() {
	persons := []Person{
		{ID: 1, Name: "김철수", Email: "kim@example.com", Age: 30},
		{ID: 2, Name: "이영희", Email: "lee@example.com", Age: 25},
		{ID: 3, Name: "박민수", Email: "park@example.com", Age: 35},
	}

	// JSON 파일로 저장
	err := SaveArrayToJSONFile(persons, "persons.json")
	if err != nil {
		fmt.Printf("에러 발생: %v\n", err)
		return
	}

	err = SaveArrayToJSONFileCompact(persons, "persons_compact.json")
	if err != nil {
		fmt.Printf("에러 발생: %v\n", err)
		return
	}
}

func SaveArrayToJSONFile[T any](data []T, filename string) error {
	// JSON으로 변환 (예쁘게 들여쓰기)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 변환 실패: %v", err)
	}

	// 파일에 저장
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("파일 저장 실패: %v", err)
	}

	return nil
}

// SaveArrayToJSONFileCompact : 한줄짜리로 할때
func SaveArrayToJSONFileCompact[T any](data []T, filename string) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON 변환 실패: %v", err)
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("파일 저장 실패: %v", err)
	}

	return nil
}
