package main

import (
	"fmt"
	"time"
)

func main() {
	startDate := "20240416"
	endDate := "20240401"

	result := createRegdateFields(startDate, endDate)

	for _, item := range result {
		fmt.Println(item)
	}
}

func createRegdateFields(startDate, endDate string) []string {
	var result []string

	start, startErr := time.Parse("20060102", startDate)
	if startErr != nil {
		fmt.Println("Error parsing start date:", startErr)
		return nil
	}

	end, endErr := time.Parse("20060102", endDate)
	if endErr != nil {
		fmt.Println("Error parsing end date:", endErr)
		return nil
	}

	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		date := current.Format("20060102")
		result = append(result, date)
	}

	return result
}
