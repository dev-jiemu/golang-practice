package main

import (
	"fmt"
	"time"
)

//func main() {
//	done := make(chan bool, 2) // 버퍼가 1개보다 클경우 비동기 채널이 생성됨
//	count := 4
//
//	go func() {
//		for i := 0; i < count; i++ {
//			done <- true
//			fmt.Println("고루틴 : ", i)
//			//time.Sleep(1 * time.Second)
//		}
//	}()
//
//	for i := 0; i < count; i++ {
//		result := <-done // 채널에 값이 들어올때까지 대기하고, 들어오면 값을 꺼냄
//		fmt.Println("메인 함수 : ", i)
//		fmt.Println("result : ", result)
//	}
//}

func num(a, b int) <-chan int {
	out := make(chan int)
	go func() {
		out <- a
		out <- b
		close(out)
	}()

	return out
}

func sum(c <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		r := 0
		for i := range c {
			r = r + i
		}
		out <- r
	}()
	return out
}

func main() {
	// 단방향 채널
	c := num(1, 3)
	out := sum(c)

	fmt.Println(<-out)

	// select
	c1 := make(chan int)
	c2 := make(chan string)

	go func() {
		for {
			c2 <- "hello :)"
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case i := <-c1:
				fmt.Println("c1 : ", i)
			case s := <-c2:
				fmt.Println("c2 : ", s)
			}
		}
	}()

	time.Sleep(5 * time.Second)
}
