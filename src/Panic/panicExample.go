package main

import "fmt"

// panic : 문법오류 상관없이 개발자가 에러처리 하고 싶을 때
// recover : try-catch랑 비슷한데, 패닉이 발생했을 때 실행할 로직임
func main() {

	//go 자체적으로 panic 날 때 상황
	/*
		a := [...]int{1, 2}
		for i := 0; i< 3; i++{
			fmt.Println(a[i]) //세번째 인덱스에서 panic 발생
		}
	*/

	//내가 에러처리 할 수도 있엉
	//panic("selp error")

	//패닉이 발생했지만 밑의 함수까지 같이 실행
	f()
	fmt.Println("hello world")

}

func f() {
	defer func() {
		s := recover()
		fmt.Println(s)
	}()

	panic("error")
}
