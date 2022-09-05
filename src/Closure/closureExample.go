package main

import "fmt"

func main() {

	a, b := 3, 5

	//익명함수 내에 선언된 변수 말고 외부에 있는 변수도 사용 가능
	f := func(x int) int {
		return a*x + b
	}

	y := f(10)

	fmt.Println("closure func data : ", y)

	cl := calc()
	fmt.Println("Closure2 func data : ", cl(3))
	fmt.Println("Closure2 func data : ", cl(4))
	fmt.Println("Closure2 func data : ", cl(5))
	fmt.Println("Closure2 func data : ", cl(6))

}

// 클로저를 사용하는 이유
// 클로저는 함수가 선언될 때 환경을 그대로 유지함
func calc() func(x int) int {
	a, b := 7, 10 //지역변수는 본래 함수가 끝나면 소멸되어야 함
	return func(x int) int {
		return a*x + b //클로저라서 calc 함수 호출할 때 마다 a, b값 사용할 수 있음
	}
}
