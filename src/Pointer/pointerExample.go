package main

import "fmt"

// 포인터랑 친해져봅시다 ㅠㅠ
func main() {

	var numPtr *int
	fmt.Println(numPtr) //포인터형 변수를 선언하면 기본 nil로 초기화

	//빈 포인터형 변수는 바로 사용할 수 없으니까 new로 메모리 할당 처리
	numPtr = new(int)
	fmt.Println(numPtr)

	//포인터에 값을 대입하고 싶으면 역참조 하면됨
	*numPtr = 1
	fmt.Println(numPtr)

	var num int = 1
	numPtr = &num
	fmt.Println("numPtr : ", numPtr)
	fmt.Println("num : ", &num)

}
