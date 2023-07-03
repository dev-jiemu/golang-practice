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

// dynamodbav tag
// dynamodbattribute로 marshal, unmarshal 할땐 dynamodav tag만 처리함
type TestTable struct {
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	SubItemList  []SubItem  `json:"sub_item_list,omitempty"`
	InfoItemList []InfoItem `json:"info_item_list,omitempty"`
}

type SubItem struct {
	SubId    string `json:"sub_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	value    string `json:"value,omitempty"`
	Size     int    `json:"size,omitempty"`
}

type InfoItem struct {
	Location string `json:"location,omitempty"`
	Value    string `json:"value,omitempty"`
}

var sampleData = `
{
  "id": "temp2",
  "name": "temp2",
  "sub_item_list": [
    {
      "sub_id": "test version 2"
    },
	{
      "size": 64
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
