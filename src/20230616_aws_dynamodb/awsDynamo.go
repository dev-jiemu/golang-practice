package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"log"
	"os"
)

type TestTable struct {
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	SubItemList  []SubItem  `json:"sub_item_list"`
	InfoItemList []InfoItem `json:"info_item_list"`
}

type SubItem struct {
	SubId    string `json:"sub_id"`
	Nickname string `json:"nickname"`
	value    string `json:"value"`
}

type InfoItem struct {
	Location string `json:"location"`
	Value    string `json:"value"`
}

var sampleData = `
{
  "id": "jiemu",
  "name": "jiemu",
  "sub_item_list": [
    {
      "value": "test"
    },
	{
      "sub_id": "food"
    }
  ]
}
`

var insertYn = "N"

func main() {
	session, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-northeast-1"),
	})
	if err != nil {
		log.Fatal("Create Session Error : ", err)
		os.Exit(1)
	}

	svc := *dynamodb.New(session)

	// Put data
	if insertYn == "Y" {
		putColumnData(&svc)
	}

	// Scan table list
	var records []TestTable

	err = svc.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String("jiemu_test_table"),
	}, func(page *dynamodb.ScanOutput, last bool) bool {
		err := dynamodbattribute.UnmarshalListOfMaps(page.Items, &records)
		if err != nil {
			panic(fmt.Sprintf("failed to unmarshal Dynamodb Scan Items, %v", err))
		}

		return true // keep paging
	})
	if err != nil {
		log.Fatal("DynamoDB scan page fail :", err)
		os.Exit(1)
	}

	jsonStr, _ := json.Marshal(records)
	log.Print("Scan table : ", string(jsonStr))

	// Get Column
	in := TestTable{
		Id:   "jiye.kim",
		Name: "jiye.kim",
	}

	_, err = dynamodbattribute.MarshalMap(in)
	if err != nil {
		log.Fatal("DynamoDB Marshalmap fail :", err)
		os.Exit(1)
	}

	params := &dynamodb.GetItemInput{
		TableName: aws.String("jiemu_test_table"),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: &in.Id,
			},
			"name": {
				S: &in.Name,
			},
		},
	}

	result, err := svc.GetItem(params)
	if err != nil {
		log.Fatal("fail get item : ", err)
		os.Exit(1)
	}

	testTable := TestTable{}
	dynamodbattribute.UnmarshalMap(result.Item, &testTable)
	jsonStr, _ = json.Marshal(testTable)
	log.Print("Scan Column : ", string(jsonStr))

	// 데이터 변경
	if testTable.Id != "" && testTable.Name != "" {
		// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/dynamo-example-update-table-item.html
	}

}

func putColumnData(svc *dynamodb.DynamoDB) {
	// Put column
	var in *TestTable
	err := json.Unmarshal([]byte(sampleData), &in)
	if err != nil {
		log.Fatal("Json unmarshal fail : ", err)
		os.Exit(1)
	}

	item, err := dynamodbattribute.MarshalMap(in)
	if err != nil {
		log.Fatal("DynamoDB Marshalmap fail :", err)
		os.Exit(1)
	}

	input := &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String("jiemu_test_table"),
	}

	_, err = svc.PutItem(input)
	if err != nil {
		log.Fatal("fail put item : ", err)
	}
}
