package main

import "fmt"

type myStruct struct {
	Field string
	Value int
}

func main() {
	mapObj := map[string]*myStruct{}

	mapObj["key1"] = &myStruct{
		Field: "key1",
		Value: 100,
	}

	fmt.Printf("key1 : %v \n", mapObj["key1"])
	fmt.Printf("key2 : %v \n", mapObj["key2"])

	if _, found := mapObj["key1"]; found {
		// 복사
		mapObj["key3"] = mapObj["key1"]
	}

	fmt.Printf("key3: %v", mapObj["key3"])
}
