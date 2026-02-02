package main

import (
	"context"
	"example/common"
)

type queueRepo struct{}

func NewQueueRepo() QueueRepository {
	return &queueRepo{}
}

func (v *queueRepo) GetCountInfo(ctx context.Context) ([]QueueCount, error) {
	var err error
	var items []QueueCount

	queryText := `
		select 
			cpk, 
			count(*) as submit_count 
		from ai_stt_queue 
		where req_uid = -1 
		group by cpk
		order by submit_count desc, cpk asc
	`

	err = common.GetDB().SelectContext(ctx, &items, queryText)
	return items, err
}
