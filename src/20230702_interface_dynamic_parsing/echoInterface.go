package main

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

	fmt.Println("===============================")
	fmt.Println("attributevalue convert")

	convert, err := ConvertToDynamoDBAttributeValue(test.Field, "field")
	if err != nil {
		fmt.Println(err.Error())
	} else {
		for key, value := range convert {
			logAttributeValue(value, key, 1)
		}
	}

	return c.String(http.StatusOK, "")
}

func logAttributeValue(attributeValue types.AttributeValue, fieldName string, indentLevel int) {
	indent := ""
	for i := 0; i < indentLevel; i++ {
		indent += "\t"
	}

	switch av := attributeValue.(type) {
	case *types.AttributeValueMemberS:
		if fieldName == "field1" || fieldName == "field2" {
			fmt.Printf("%s%s: %s\n", indent, fieldName, av.Value)
		}
	case *types.AttributeValueMemberM:
		fmt.Printf("%s%s:\n", indent, fieldName)
		for key, value := range av.Value {
			fmt.Printf("%s\t%s: ", indent, key)
			logAttributeValue(value, key, indentLevel+1)
		}
	default:
		fmt.Printf("%sUnknown field\n", indent)
	}
}

func ConvertToDynamoDBAttributeValue(field interface{}, fieldName string) (map[string]types.AttributeValue, error) {
	attributeValue := make(map[string]types.AttributeValue)

	switch fieldValue := field.(type) {
	case string:
		attributeValue[fieldName] = &types.AttributeValueMemberS{Value: fieldValue}
	case map[string]interface{}:
		fieldMap, err := attributevalue.MarshalMap(fieldValue)
		if err == nil {
			attributeValue[fieldName] = &types.AttributeValueMemberM{
				Value: fieldMap,
			}
		} else {
			return attributeValue, err
		}
	case []interface{}:
		fieldList, err := attributevalue.MarshalList(fieldValue)
		if err == nil {
			attributeValue[fieldName] = &types.AttributeValueMemberL{
				Value: fieldList,
			}
		} else {
			return attributeValue, err
		}
	default:
		return attributeValue, errors.New("unsupported field type")
	}

	return attributeValue, nil
}
