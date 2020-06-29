package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/xumc/go-dynamodb-stream-subscriber/stream"
	"os"
	"strings"
	"sync"
)

type DDBStreamMonitor struct{
	tableChan chan <- OutputItem
}

func (ds *DDBStreamMonitor) Monitor(tables []string) {
	streamSvc, dynamoSvc := initDDB()

	checkTables(tables, dynamoSvc)

	enableStreams(tables, dynamoSvc)

	wg := sync.WaitGroup{}
	for _, tt := range tables {
		wg.Add(1)
		go tailDDBTable(tt, streamSvc, dynamoSvc, ds.tableChan)
	}
}

func enableStreams(tables []string, dynamoSvc *dynamodb.DynamoDB) {
	for _, tt := range tables {
		dto, err := dynamoSvc.DescribeTable(&dynamodb.DescribeTableInput{TableName: aws.String(tt)})
		if err != nil{
			fmt.Println(err)
			os.Exit(1)
		}

		if dto.Table.StreamSpecification != nil && *dto.Table.StreamSpecification.StreamEnabled && *dto.Table.StreamSpecification.StreamViewType == dynamodb.StreamViewTypeNewAndOldImages {
			continue
		}

		_, err = dynamoSvc.UpdateTable(&dynamodb.UpdateTableInput{
			StreamSpecification: &dynamodb.StreamSpecification{
				StreamEnabled: aws.Bool(true),
				StreamViewType: aws.String(dynamodb.StreamViewTypeNewAndOldImages),
			},
			TableName: aws.String(tt),
		})
		if err != nil{
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

func checkTables(tables []string, dynamoSvc *dynamodb.DynamoDB) {
	result, err := dynamoSvc.ListTables(&dynamodb.ListTablesInput{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	notExistTables := make([]string, 0)
	for _, tt := range tables {
		hasTable := false
		for _, t := range result.TableNames {
			if tt == *t {
				hasTable = true
				break
			}
		}
		if !hasTable {
			notExistTables = append(notExistTables, tt)
		}
	}
	if len(notExistTables) > 0 {
		fmt.Printf("tables %s doesn't exist", strings.Join(notExistTables, ","))
		os.Exit(1)
	}
}

func tailDDBTable(table string, streamSvc *dynamodbstreams.DynamoDBStreams, dynamoSvc *dynamodb.DynamoDB, outputer chan <- OutputItem) {
	streamSubscriber := stream.NewStreamSubscriber(dynamoSvc, streamSvc, table)
	ch, errCh := streamSubscriber.GetStreamDataAsync()

	go func(errCh <-chan error) {
		for err := range errCh {
			fmt.Println("Stream Subscriber error: ", err)
			os.Exit(1)
		}
	}(errCh)

	for record := range ch {
		fmt.Println(ch)
		outputer <- OutputItem{
			TableName: table,
			Action: *record.EventName,
			Changes: translateChanges(record.Dynamodb),
		}
	}
}

func translateChanges(changes *dynamodbstreams.StreamRecord) [][3]string {
	allKeysMap := make(map[string]struct{})
	for k, _ := range changes.OldImage {
		allKeysMap[k] = struct{}{}
	}
	for k, _ := range changes.NewImage {
		allKeysMap[k] = struct{}{}
	}
	allKeys := make([]string, 0)
	for k, _ := range allKeysMap {
		allKeys = append(allKeys, k)
	}

	ret := make([][3]string, len(allKeys))

	for i, k := range allKeys {
		oldVal := translateDDBValue(changes.OldImage[k])
		newVal := translateDDBValue(changes.NewImage[k])

		ret[i] = [3]string{k, oldVal, newVal }
	}

	return ret
}

func translateDDBValue(value *dynamodb.AttributeValue) string {
	if value == nil {
		return "undefined"
	}

	v := strings.ReplaceAll(value.String(), "{\n  ", "{")
	v = strings.ReplaceAll(v, "\n}", "}")
	return v
}

func initDDB() (*dynamodbstreams.DynamoDBStreams, *dynamodb.DynamoDB){
	sess, err := session.NewSession(&aws.Config{
		Endpoint:    aws.String("http://localhost:8000"),
		Region:      aws.String("local"),
		Credentials: credentials.NewStaticCredentials("a", "b", "c"),
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	streamSvc := dynamodbstreams.New(sess)
	dynamoSvc := dynamodb.New(sess)

	return streamSvc, dynamoSvc
}
