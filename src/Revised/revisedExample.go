package main

import "fmt"

/*
func sum(n ...int) int {
	total := 0

	for _, value := range n {
		total += value
	}

	return total
}*/

func factorial(n uint64) uint64 {
	if n == 0 {
		return 1
	}

	return n * factorial(n-1)
}

func sum(a int, b int) int {
	return a + b
}

func diff(a int, b int) int {
	return a - b
}

// 가변인자를 테스트 해보겠다
func main() {

	//sample := sum(1, 2, 3, 4, 5, 6, 7)
	//fmt.Println("sample data : ", sample)

	//가변인자는 슬라이스 타입이라서 아래처럼 넘겨줄 수 있음
	//n := []int{1, 2, 3, 4, 5}
	//sample2 := sum(n...)
	//fmt.Println("sample2 data : ", sample2)

	//팩토리얼
	factorial := factorial(5)
	fmt.Println("factorial data : ", factorial)

	//함수를 변수에 저장해보겠다
	//world := sum //이런식으로 간략하게 넣을 수도 있음
	//fmt.Println("world data : ", world(1, 2, 3))

	//map에 함수 넣는것도 가능
	f := map[string]func(int, int) int{
		"sum":  sum,
		"diff": diff,
	}

	fmt.Println("sum data : ", f["sum"](1, 3))
	fmt.Println("diff data : ", f["diff"](5, 1))

	//익명함수 선언 후 호출하기
	func(s string) {
		fmt.Println(s)
	}("hello XD")

	t := func(a int, b int) int {
		return a * b
	}(3, 7)
	fmt.Println("t data : ", t)

}
