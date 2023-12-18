package main

import (
	"fmt"
	"time"
)

func main() {
	// 작업 시작 시간 기록
	startTime := time.Now()

	// 여기에 작업을 수행하는 코드를 넣으세요.
	time.Sleep(2 * time.Second)

	// 작업 종료 시간 기록
	endTime := time.Now()

	// 소요 시간 계산
	duration := endTime.Sub(startTime)

	// 소요 시간 출력
	fmt.Printf("작업 소요 시간: %s\n", duration)
}
