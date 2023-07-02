package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

type bodyDynamicParsing struct {
	Field interface{} `json:"field"`
}

func main() {

	e := echo.New()

	// Tester API Endpoint
	e.POST("/interface", interfaceAPI)

	// 서버 시작
	e.Start(":9000")
}

func interfaceAPI(c echo.Context) error {
	var err error

	var test *bodyDynamicParsing

	err = c.Bind(&test)
	if err != nil {
		log.Fatalf("bind fail : %v", err)
		return c.String(http.StatusBadRequest, "bad request")
	}

	log.Printf("request : %v", test.Field)

	switch fieldValue := test.Field.(type) {
	case string:
		// string 타입인 경우
		fmt.Println("Field is a string:", fieldValue)

	case map[string]interface{}:
		// map 타입인 경우
		fmt.Println("Field is a map:", fieldValue)

	case []interface{}:
		// 배열 타입인 경우
		fmt.Println("Field is an array:", fieldValue)

	default:
		// 다른 타입인 경우
		fmt.Println("Field has an unsupported type")
	}

	return c.String(http.StatusOK, "")
}
