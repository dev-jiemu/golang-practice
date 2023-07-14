package main

import (
	"fmt"
	"time"
)

func main() {
	// 오늘 날짜 가져오기
	now := time.Now()

	// 지난주 월요일과 일요일의 YYYYMMDD 문자열 데이터 추출
	lastMonday := now.AddDate(0, 0, -int(now.Weekday())-6)
	lastSunday := now.AddDate(0, 0, -int(now.Weekday()))
	mondayStr := lastMonday.Format("20060102")
	sundayStr := lastSunday.Format("20060102")
	fmt.Println("지난주 월요일:", mondayStr)
	fmt.Println("지난주 일요일:", sundayStr)

	// 지난달 1일과 말일의 YYYYMMDD 문자열 데이터 추출
	firstDay := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	lastDay := firstDay.AddDate(0, 1, -1)
	firstDayStr := firstDay.Format("20060102")
	lastDayStr := lastDay.Format("20060102")
	fmt.Println("지난달 1일:", firstDayStr)
	fmt.Println("지난달 말일:", lastDayStr)
}
