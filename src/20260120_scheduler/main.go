package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	ProcessingCount int // Processing 상태로 발행할 개수
	PendingCount    int // Pending 상태로 발행할 개수

	MaxDedicatedUsers     int           // 최대 몇 명까지 할당할지 (예: 3명)
	DedicatedQuotaPercent float64       // 선점 영역 비율 (예: 0.25 = 25%)
	StatRefreshInterval   time.Duration // 통계 갱신 주기 (RDS)
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	config := &Config{
		ProcessingCount:       20,
		PendingCount:          4,
		MaxDedicatedUsers:     3,
		DedicatedQuotaPercent: 0.25, // 선점 영역 25%
		StatRefreshInterval:   5 * time.Second,
	}

	scheduler := NewScheduler(config)

	setupTestData(scheduler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runDynamicScenarios(ctx, scheduler)

	// 10초간 실행 (테스트용)
	go func() {
		time.Sleep(10 * time.Second)
		log.Println("\n=== Test completed ===")
		cancel()
	}()

	// monitoring
	go monitorStatus(ctx, scheduler)

	scheduler.Start(ctx)
}

// setupTestData : 초기 테스트 데이터 (현실적인 시나리오)
func setupTestData(s *Scheduler) {
	log.Println("\n=== Setting up realistic test data ===")
	log.Println("Simulating real-world scenario:")
	log.Println("  - ProcessingCount=20, PendingCount=4")
	log.Println("  - MaxDedicatedUsers=3, DedicatedQuotaPercent=0.25")
	log.Println("  - Distribution: ProcessingCount -> PendingCount order")
	log.Println("")

	log.Println("Creating user1 with 5000 pending tasks (heavy user)")
	for i := 0; i < 5000; i++ {
		s.AddTask(&Task{
			ID:     fmt.Sprintf("user1-task-%d", i),
			UserID: "user1",
			Status: "pending",
		})
	}

	log.Println("Creating user2 with 3000 pending tasks (heavy user)")
	for i := 0; i < 3000; i++ {
		s.AddTask(&Task{
			ID:     fmt.Sprintf("user2-task-%d", i),
			UserID: "user2",
			Status: "pending",
		})
	}

	log.Println("Creating user3 with 500 pending tasks")
	for i := 0; i < 500; i++ {
		s.AddTask(&Task{
			ID:     fmt.Sprintf("user3-task-%d", i),
			UserID: "user3",
			Status: "pending",
		})
	}

	// 초기 통계 갱신
	s.refreshStats()
	s.recalculateQuotas()

	log.Println("\n=== Initial test data setup completed ===")
	log.Println("Expected behavior:")
	log.Println("  - 3 users (≤ MaxDedicatedUsers) → equal distribution")
	log.Println("  - ProcessingCount: 20/3 ≈ 6-7 each")
	log.Println("  - PendingCount: 4/3 ≈ 1-2 each")
	printCurrentStats(s)
}

func runDynamicScenarios(ctx context.Context, s *Scheduler) {

	// 시나리오 1: 3초 후 네 번째 대량 유저 등장
	time.Sleep(3 * time.Second)
	select {
	case <-ctx.Done():
		return
	default:
		log.Println("\n=== [3s] Scenario: 4th heavy user arrives ===")
		for i := 0; i < 4000; i++ {
			s.AddTask(&Task{
				ID:     fmt.Sprintf("user4-task-%d", i),
				UserID: "user4",
				Status: "pending",
			})
		}
		log.Println("Added user4 with 4000 pending tasks")
		log.Println("Expected: 4 users, but MaxDedicatedUsers=3")
		log.Println("  → Top 3 users by pending count get dedicated slots (25% each)")
		log.Println("  → user4 competes in shared pool")

		// 통계 즉시 갱신 (새 유저 반영)
		s.refreshStats()
		s.recalculateQuotas()
	}

	// 시나리오 2: 6초 후 다섯 번째 대량 유저 등장
	time.Sleep(3 * time.Second)
	select {
	case <-ctx.Done():
		return
	default:
		log.Println("\n=== [6s] Scenario: 5th heavy user arrives ===")
		for i := 0; i < 3500; i++ {
			s.AddTask(&Task{
				ID:     fmt.Sprintf("user5-task-%d", i),
				UserID: "user5",
				Status: "pending",
			})
		}
		log.Println("Added user5 with 3500 pending tasks")
		log.Println("Expected: 5 users, MaxDedicatedUsers=3")
		log.Println("  → Top 3 by pending count keep dedicated slots")
		log.Println("  → user4, user5 share remaining slots (least requests first)")

		// 통계 즉시 갱신
		s.refreshStats()
		s.recalculateQuotas()
	}
}

// completeUserTasks : 특정 유저의 task 완료 처리 (시뮬레이션용)
func completeUserTasks(s *Scheduler, userID string, count int) {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()

	completed := 0
	for _, task := range s.tasks {
		if task.UserID == userID && (task.Status == "Processing" || task.Status == "Pending") && completed < count {
			task.Status = "completed"
			completed++
		}
	}

	log.Printf("✓ Simulated completion of %d tasks for %s", completed, userID)
}

// monitorStatus : 실시간 상태 모니터링
func monitorStatus(ctx context.Context, s *Scheduler) {
	ticker := time.NewTicker(10 * time.Second) // 10초마다 모니터링
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := int(time.Since(startTime).Seconds())
			log.Printf("\n=== [%ds] Current Status ===", elapsed)
			printCurrentStats(s)
			printUserQuotas(s)
		}
	}
}

// printCurrentStats : 현재 통계 출력
func printCurrentStats(s *Scheduler) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	log.Println("User Statistics:")

	// pending 많은 순으로 정렬해서 출력
	type userStatPair struct {
		userID string
		stat   *UserStat
	}

	pairs := make([]userStatPair, 0, len(s.userStats))
	for userID, stat := range s.userStats {
		pairs = append(pairs, userStatPair{userID, stat})
	}

	// pending 내림차순 정렬
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].stat.PendingCount < pairs[j].stat.PendingCount {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	for _, pair := range pairs {
		userID := pair.userID
		stat := pair.stat

		log.Printf("  %s: pending=%d, queued=%d",
			userID, stat.PendingCount, stat.RunningCount)
	}
}

// printUserQuotas : 할당량 현황 출력
func printUserQuotas(s *Scheduler) {
	s.quotaMu.RLock()
	defer s.quotaMu.RUnlock()

	if len(s.userQuotas) == 0 {
		log.Println("User Quotas: (none - all shared)")
		return
	}

	log.Printf("User Quotas (%d users allocated, shared_quota=%d):",
		len(s.userQuotas), s.quotaForShared)

	// quota 큰 순으로 정렬
	type quotaPair struct {
		userID string
		quota  *UserQuota
	}

	pairs := make([]quotaPair, 0, len(s.userQuotas))
	for userID, quota := range s.userQuotas {
		pairs = append(pairs, quotaPair{userID, quota})
	}

	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[i].quota.MaxSlots < pairs[j].quota.MaxSlots {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	for _, pair := range pairs {
		log.Printf("  %s: max_slots=%d",
			pair.userID, pair.quota.MaxSlots)
	}
}
