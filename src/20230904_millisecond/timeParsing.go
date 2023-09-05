package main

import (
	"fmt"
	"time"
)

func main() {
	timestampMilliseconds := int64(1693245600000)
	timestampSeconds := timestampMilliseconds / 1000
	t := time.Unix(timestampSeconds, 0)
	formattedTime := t.Format("2006-01-02 15:04:05") // 원하는 형식으로 포맷팅
	fmt.Println(formattedTime)

	// 현재 시간을 가져옵니다.
	currentTime := time.Now()

	// 1시간을 빼고 분 단위를 00으로 절삭합니다.
	oneHourAgo := currentTime.Add(-1 * time.Hour)
	oneHourAgo = oneHourAgo.Truncate(time.Hour)

	// 결과를 출력합니다.
	fmt.Println("현재 시간:", currentTime.Format("15시 04분"))
	fmt.Println("1시간 전의 시간:", oneHourAgo.Format("15시 04분"))

	// 현재 시간을 가져옵니다.
	currentTimeNow := time.Now().Format("2006-01-02")
	oneWeek, _ := time.Parse("2006-01-02", currentTimeNow)

	// 7일전
	startDate := oneWeek.AddDate(0, 0, -7)

	// 종료 날짜를 현재 시간으로 설정하고 1초를 빼서 23:59:59로 만듭니다.
	endDate := oneWeek.Add(-time.Second)

	// 결과를 출력합니다.
	fmt.Println("1주일 전:", oneWeek.Format("2006-01-02"))
	fmt.Println("시작 날짜:", startDate.Format("2006-01-02 15:04:05"))
	fmt.Println("종료 날짜:", endDate.Format("2006-01-02 15:04:05"))
}
