package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

func TestSimple() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	config := &Config{
		ProcessingCount:       20,
		PendingCount:          4,
		MaxDedicatedUsers:     3,
		DedicatedQuotaPercent: 0.25,
		StatRefreshInterval:   5 * time.Second,
	}

	scheduler := NewScheduler(config)

	// 간단한 테스트 데이터
	fmt.Println("=== Creating test data ===")
	for i := 0; i < 100; i++ {
		scheduler.AddTask(&Task{
			ID:     fmt.Sprintf("user1-task-%d", i),
			UserID: "user1",
			Status: "pending",
		})
	}
	for i := 0; i < 50; i++ {
		scheduler.AddTask(&Task{
			ID:     fmt.Sprintf("user2-task-%d", i),
			UserID: "user2",
			Status: "pending",
		})
	}
	for i := 0; i < 30; i++ {
		scheduler.AddTask(&Task{
			ID:     fmt.Sprintf("user3-task-%d", i),
			UserID: "user3",
			Status: "pending",
		})
	}

	scheduler.refreshStats()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go scheduler.Start(ctx)

	time.Sleep(3 * time.Second)
	fmt.Println("\n=== Stats after 3 seconds ===")
	printCurrentStats(scheduler)

	time.Sleep(7 * time.Second)
	fmt.Println("\n=== Test completed ===")
}
