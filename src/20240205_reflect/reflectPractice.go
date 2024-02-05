package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

type MyStruct struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	List   bool   `json:"list"`
}

func main() {
	// JSON 데이터를 읽어옴
	jsonData := `{
		"field1": {
			"number": 3,
			"name": "test"
		},
		"field2": {
			"number": "10",
			"list": false
		},
		"field3": {
			"number": 50,
			"name": "object"
		}
	}`

	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// 구조체 타입으로 변환
	for key, value := range data {
		// JSON을 다시 마샬링하여 구조체로 변환
		jsonBytes, _ := json.Marshal(value)
		var myStruct MyStruct
		err := json.Unmarshal(jsonBytes, &myStruct)
		if err != nil {
			fmt.Println("Error decoding JSON to struct:", err)

			if subMap, ok := value.(map[string]interface{}); ok {
				structType := reflect.TypeOf(myStruct)
				structValue := reflect.ValueOf(&myStruct).Elem()

				for i := 0; i < structValue.NumField(); i++ {
					field := structType.Field(i)
					fieldName := field.Tag.Get("json")
					fieldValue := structValue.Field(i)

					fmt.Printf("field: %+v, fieldName: %v\n", fieldValue, fieldName)

					if fieldValue.Kind() != reflect.Struct {
						if subValue, subOk := subMap[fieldName]; subOk {
							setFieldValue(field.Type.Kind(), subValue, fieldName, &myStruct)
						}
					}
				}
			}
		}

		// 결과 출력
		fmt.Printf("%s: %+v\n", key, myStruct)
	}
}

func setFieldValue(field reflect.Kind, value interface{}, fieldName string, myStruct *MyStruct) {
	interfaceType := reflect.TypeOf(value)
	interfaceValue := reflect.ValueOf(value)
	fmt.Printf("fieldName: %v, originalType : %v, interfaceValueType : %+v\n", fieldName, field, interfaceType)

	// 구조체가 포인터인 경우 포인터를 역참조
	structValue := reflect.ValueOf(myStruct).Elem()

	// 태그에서 json 태그명을 사용해서 필드를 찾기
	var fieldValue reflect.Value
	for i := 0; i < structValue.NumField(); i++ {
		field := structValue.Type().Field(i)
		if field.Tag.Get("json") == fieldName {
			fieldValue = structValue.Field(i)
			break
		}
	}

	if field != interfaceType.Kind() {
		switch interfaceType.Kind() {
		case reflect.String:
			intValue, _ := strconv.Atoi(interfaceValue.String())
			fieldValue.SetInt(int64(intValue))
		default:
			break
		}
	}
}
