package main

import "fmt"

type DateAndCount struct {
	Date       string
	Count      int64
	UsingCount int64
}

func main() {
	datetime := []DateAndCount{
		{Date: "20230925", Count: 17, UsingCount: 55},
		{Date: "20230925", Count: 15, UsingCount: 2},
		{Date: "20231010", Count: 14, UsingCount: 0},
		{Date: "20231017", Count: 144, UsingCount: 17},
	}

	// Date별 Count와 UsingCount를 저장하기 위한 맵 생성
	countByDate := make(map[string]DateAndCount)

	// datetime 슬라이스를 순회하면서 Date별 Count와 UsingCount를 누적
	for _, data := range datetime {
		entry := countByDate[data.Date]
		entry.Count += data.Count
		entry.UsingCount += data.UsingCount
		countByDate[data.Date] = entry
	}

	// 결과 출력
	for date, counts := range countByDate {
		fmt.Printf("Date: %s, Total Count: %d, Total UsingCount: %d\n", date, counts.Count, counts.UsingCount)
	}
}
