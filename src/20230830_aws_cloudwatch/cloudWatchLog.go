package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/aws"
	"time"
)

// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2@v0.24.0/service/cloudwatchlogs/cloudwatchlogsiface
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2@v0.24.0/service/cloudwatchlogs#FilterLogEventsInput
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html#changing-default-increment-value
func main() {
	logGroupName := ""

	config, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		fmt.Println("LoadDefaultConfig error : ", err)
	}

	client := cloudwatchlogs.NewFromConfig(config)

	//listStreamsInput := &cloudwatchlogs.DescribeLogStreamsInput{
	//	LogGroupName: &logGroupName,
	//}
	//
	//streamsResp, err := client.DescribeLogStreams(context.TODO(), listStreamsInput)
	//if err != nil {
	//	fmt.Println("Could not list log streams")
	//	fmt.Println(err)
	//	return
	//}
	//
	//logStreamNameArray := make([]string, len(streamsResp.LogStreams))
	//for idx, stream := range streamsResp.LogStreams {
	//	logStreamNameArray[idx] = *stream.LogStreamName
	//	fmt.Println("LogStreamName : ", *stream.LogStreamName)
	//}
	//fmt.Println("total count : ", len(logStreamNameArray))

	filterPattern := "{ $.field_name = \"field_value\" }"

	logStreamNamePrefix := ""

	now := time.Now()
	startTime := now.Add(-time.Hour*24*7).Unix() * 1000
	endTime := now.Unix() * 1000

	filter := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &logGroupName,
		StartTime:           &startTime,
		EndTime:             &endTime,
		LogStreamNamePrefix: &logStreamNamePrefix,
		//LogStreamNames: logStreamNameArray,
		FilterPattern: &filterPattern,
		Limit:         aws.Int32(200),
	}

	for {
		filterResp, err := client.FilterLogEvents(context.TODO(), filter)
		if err != nil {
			fmt.Println("Could not fetch log events for stream:")
			fmt.Println(err)
		}

		for _, event := range filterResp.Events {
			fmt.Println(*event.Message)
			fmt.Println("current stream name : ", *event.LogStreamName)
		}

		fmt.Println("total count : ", len(filterResp.Events))
		fmt.Println("===============================")

		if filterResp.NextToken == nil {
			fmt.Println("No more results")
			break // nextToken 없어서 종료
		} else {
			filter.NextToken = filterResp.NextToken
		}
	}

}
