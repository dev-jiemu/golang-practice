package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("BatchService Start ====")

	// 현재 시간
	now := time.Now()
	nextRun := now.Add(time.Hour).Truncate(30 * time.Minute)

	// 다음 시간까지 대기
	durationUntilNextRun := nextRun.Sub(now)
	fmt.Printf("다음 실행 시간까지 대기 중 (%s 후에 실행됩니다)...\n", durationUntilNextRun)
	time.Sleep(durationUntilNextRun)

	// 최초 실행 : 이후 ticker로 주기적인 실행
	fmt.Println("process 가동")
	fmt.Printf("now time : %v\n", time.Now())

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

			case <-tickerQuitCh:
				ticker.Stop()
				return
			}
		}
	}()

	select {}
}
