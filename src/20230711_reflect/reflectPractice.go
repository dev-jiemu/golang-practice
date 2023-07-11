package main

import (
	"fmt"
	"reflect"
)

type testStruct struct {
	StringField  string
	IntField     int
	BoolField    bool
	Float64Field float64
}

func main() {

	test := testStruct{
		StringField: "test",
		BoolField:   true, // false가 zero value였네 ㅇㅂㅇ...
	}

	count := 0

	// 구조체 값이 존재하는 필드 검색
	structValue := reflect.ValueOf(test)
	structType := structValue.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := structType.Field(i)
		fieldName := fieldType.Name

		// zero value 확인
		isZero := reflect.DeepEqual(field.Interface(), reflect.Zero(field.Type()).Interface())

		// true면 값이 없음
		if isZero {
			fmt.Printf("Field '%s' in MyStruct is zero value\n", fieldName)
		} else {
			count = count + 1
		}
	}

	fmt.Printf("count : %d", count)

}
