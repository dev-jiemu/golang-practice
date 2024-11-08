package main

import "fmt"

func main() {
	// map 선언 및 초기화
	myMap := map[string]int{
		"apple":  5,
		"banana": 3,
		"orange": 7,
	}

	fmt.Printf("Map의 총 key 개수: %d\n", len(myMap))

	delete(myMap, "apple") // 아예 메모리 상에서 삭제함

	fmt.Printf("Map의 총 key 개수: %d\n", len(myMap))
}
