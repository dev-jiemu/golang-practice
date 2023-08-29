package main

import (
	"fmt"
	"sort"
)

type Person struct {
	Name string
	Age  int
}

// 2023.08.29 객체 차이 복습
func main() {
	originalSlice := []*int{new(int), new(int), new(int)}
	newValue := new(int)
	appendedSlice := append(originalSlice, newValue) // append는 결과적으로 다른 객체를 리턴한다

	fmt.Printf("Original Slice: %p\n", originalSlice)
	fmt.Printf("Appended Slice: %p\n", appendedSlice)

	people := []Person{
		{"Alice", 30},
		{"Bob", 25},
		{"Eve", 35},
	}

	fmt.Printf("Original people: %p\n", people)
	sort.Slice(people, func(i, j int) bool {
		return people[i].Age < people[j].Age
	}) // 포인터 배열을 넣던 아니던 반환값이 없으므로 내부적으로 처리 :: 즉, 같은 객체임

	fmt.Printf("Sorted people: %p\n", people)
	fmt.Println("Sorted People:", people)

}
