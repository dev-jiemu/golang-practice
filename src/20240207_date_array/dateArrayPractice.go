package main

import (
	"fmt"
	"time"
)

func main() {
	startDate := "2024020100"
	endDate := "2024020523"
	selectType := "time"

	result := createDateFields(startDate, endDate, selectType)

	for k, v := range result {
		fmt.Printf("item[%v] : %v\n", k, v)
	}
}

func createDateFields(startDate, endDate, selectType string) []string {
	result := make([]string, 0)

	var layout string
	if selectType == "date" {
		layout = "20060102"
	} else if selectType == "time" {
		layout = "2006010215"
	} else if selectType == "month" {
		layout = "200601"
	}

	start, startErr := time.Parse(layout, startDate)
	if startErr != nil {
		fmt.Println("Error parsing start date:", startErr)
		return nil
	}

	end, endErr := time.Parse(layout, endDate)
	if endErr != nil {
		fmt.Println("Error parsing end date:", endErr)
		return nil
	}

	if selectType == "date" {
		for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
			currentDate := current.Format(layout)
			result = append(result, currentDate)
		}
	} else if selectType == "time" {
		for current := start; !current.After(end); current = current.Add(60 * time.Minute) {
			currentDate := current.Format(layout)
			result = append(result, currentDate)
		}
	} else if selectType == "month" {
		for current := start; !current.After(end); current = current.AddDate(0, 1, 0) {
			currentDate := current.Format(layout)
			result = append(result, currentDate)
		}
	}

	return result
}
