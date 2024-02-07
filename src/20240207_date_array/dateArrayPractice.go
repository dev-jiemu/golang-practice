package main

import (
	"fmt"
	"time"
)

type dateRange struct {
	startDate string
	endDate   string
}

func main() {
	startDate := "202311"
	endDate := "202401"
	selectType := "month"

	result := createDateFields(startDate, endDate, selectType)

	for k, v := range result {
		fmt.Printf("item[%v] : %+v\n", k, v)
	}
}

func createDateFields(startDate, endDate, selectType string) []dateRange {
	result := make([]dateRange, 0)

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

	// 시작과 끝 년도 계산
	startYear := start.Year()
	endYear := end.Year()

	for year := startYear; year <= endYear; year++ {
		yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		yearEnd := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)

		// 범위의 끝이 원래의 끝 날짜를 넘어가면 수정
		if yearEnd.After(end) {
			yearEnd = end
		}

		// 첫 해는 시작 월부터, 이후 해들은 1월 1일부터 시작
		if year == startYear {
			yearStart = start
		}

		d := dateRange{
			startDate: yearStart.Format(layout),
			endDate:   yearEnd.Format(layout),
		}
		result = append(result, d)
	}

	return result
}
