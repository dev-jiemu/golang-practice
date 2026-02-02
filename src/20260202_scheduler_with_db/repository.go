package main

import "context"

// QueueCount model 로 별도로 관리해야하는데 어차피 POC 코드니까 뭉침 ㅇㅂㅇ
type QueueCount struct {
	Cpk         string `json:"cpk" db:"cpk"`
	SubmitCount int64  `json:"submit_count" db:"submit_count"`
}

type QueueRepository interface {
	GetCountInfo(ctx context.Context) ([]QueueCount, error)
}
