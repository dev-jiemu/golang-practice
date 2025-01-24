package main

import (
	"fmt"
	"time"
)

func main() {
	current := time.Now().Add(-1 * time.Hour).Truncate(time.Hour)

	var startDate, endDate string
	var startTime, endTime time.Time

	dateType := "month"

	if dateType == "hour" { // 직전 1시간
		startTime = time.Date(current.Year(), current.Month(), current.Day(), current.Hour(), 0, 0, 0, current.Location())
		endTime = time.Date(current.Year(), current.Month(), current.Day(), current.Hour(), 23, 59, 999, current.Location())
	} else if dateType == "daily" { // 전날
		yesterday := current.AddDate(0, 0, -1)
		startTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		endTime = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999, yesterday.Location())
	} else if dateType == "month" { // 전달
		prevMonth := current.AddDate(0, -1, 0)
		startTime = time.Date(prevMonth.Year(), prevMonth.Month(), 1, 0, 0, 0, 0, prevMonth.Location())
		endTime = time.Date(prevMonth.Year(), prevMonth.Month()+1, 0, 23, 59, 59, 999, prevMonth.Location())
	}

	startDate = startTime.Format("2006-01-02 15:04:05")
	endDate = endTime.Format("2006-01-02 15:04:05")

	fmt.Println("startDate:", startDate)
	fmt.Println("endDate:", endDate)
}
