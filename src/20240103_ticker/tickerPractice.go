package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("BatchService Start ====")

	// 코드 기동시간 기준으로 다음 시간 01분부터 배치가 작동하도록 수정
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 1, 0, 0, now.Location())
	durationUntilNextRun := nextRun.Sub(now)

	time.Sleep(durationUntilNextRun)

	ticker := time.NewTicker(1 * time.Hour)
	tickerQuitCh := make(chan bool)

	go func() {
		defer func() {
			fmt.Println("BatchService STOP ====")
			ticker = nil
		}()

		for {
			select {
			case <-ticker.C:
				// process
				fmt.Println("process 가동")
				fmt.Printf("now time : %v\n", time.Now())
			// 무한루프를 위한 추가 조건
			case <-tickerQuitCh:
				ticker.Stop()
				return
			}
		}
	}()

	select {}
}
