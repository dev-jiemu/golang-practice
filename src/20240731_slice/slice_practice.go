package main

import "fmt"

// length : 배열에 들어간 실제 데이터의 길이, capacity : 현 시점에서 최대로 넣을 수 있는 길이

func main() {
	// 용량이 5인 배열 생성
	array := [5]int{1, 2, 3, 4, 5}

	// 배열로부터 길이 3, 용량 5인 슬라이스 생성
	// 용량 : slice가 참조하는 배열의 총 크기 : 5
	slice := array[0:3] // 슬라이스: [1, 2, 3]

	fmt.Println("Slice:", slice)                  // [1 2 3]
	fmt.Println("Length of slice:", len(slice))   // 3
	fmt.Println("Capacity of slice:", cap(slice)) // 5

	// 슬라이스에 요소 추가 (용량을 초과하지 않음)
	slice = append(slice, 6)
	fmt.Println("After appending 6:", slice)      // [1 2 3 6]
	fmt.Println("Length of slice:", len(slice))   // 4
	fmt.Println("Capacity of slice:", cap(slice)) // 5

	// 슬라이스에 요소 추가 (용량을 초과)
	slice = append(slice, 7, 8, 9)
	fmt.Println("After appending 7, 8, 9:", slice) // [1 2 3 6 7 8 9]
	fmt.Println("Length of slice:", len(slice))    // 7
	fmt.Println("Capacity of slice:", cap(slice))  // 10
}
