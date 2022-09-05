package main

import (
	"fmt"
	"sync"
	"time"
)

var glovalValue int

func action(i int, mutex *sync.Mutex, wg *sync.WaitGroup) {
	mutex.Lock()
	glovalValue += i
	mutex.Unlock()

	time.Sleep(1 * time.Second)

	wg.Done() //고루틴 하나가 처리되었음을 알림
}

// 고루틴, sync 관련 연습
// https://lynlab.co.kr/blog/82
func main() {
	var mutex sync.Mutex
	var wg sync.WaitGroup
	wg.Add(100) //기다릴 고루틴의 개수 설정

	startTime := time.Now()

	for i := 0; i < 100; i++ {
		go action(i, &mutex, &wg)
	}

	wg.Wait() //모든 고루틴이 실행 완료될 때까지 기다림.

	delta := time.Now().Sub(startTime)
	fmt.Printf("Result is %d, done in %.3fs\n", glovalValue, delta.Seconds())
}
