package main

import (
	"context"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Task struct {
	ID     string
	UserID string
	Status string // "pending", "Processing", "Pending", "completed"
}

type UserStat struct {
	UserID       string
	PendingCount int
	RunningCount int
	LastUpdated  time.Time
}

// UserQuota : 동적 공정분배
type UserQuota struct {
	UserID       string
	MaxSlots     int
	CurrentUsage int
	LastUpdated  time.Time
}

type Scheduler struct {
	config *Config

	// 인메모리 스토리지 (POC용)
	tasks   map[string]*Task // taskID -> Task
	tasksMu sync.RWMutex

	// ★ dispatch된 task 관리 (내부 큐 시뮬레이션)
	dispatchedTasks map[string]*Task // taskID -> Task
	dispatchedMu    sync.RWMutex

	// 통계 캐시
	userStats map[string]*UserStat
	statsMu   sync.RWMutex

	// 동적 할당량 관리
	userQuotas     map[string]*UserQuota
	quotaForShared int // 공용 영역 할당량
	quotaMu        sync.RWMutex
}

func NewScheduler(config *Config) *Scheduler {
	return &Scheduler{
		config:          config,
		tasks:           make(map[string]*Task),
		dispatchedTasks: make(map[string]*Task),
		userStats:       make(map[string]*UserStat),
		userQuotas:      make(map[string]*UserQuota),
	}
}

func (v *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 통계 갱신용 고루틴
	go v.startStatRefresher(ctx)

	// ★ Worker 시뮬레이션 고루틴 (task 완료 처리)
	go v.startWorkerSimulation(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := v.processBatch(); err != nil {
				log.Errorf("batch error: %v", err)
			}
		}
	}
}

func (v *Scheduler) processBatch() error {
	// ProcessingCount -> PendingCount 순서로 분배
	v.dispatchedMu.RLock()
	processingCount := 0
	pendingCount := 0
	for _, task := range v.dispatchedTasks {
		if task.Status == "Processing" {
			processingCount++
		} else if task.Status == "Pending" {
			pendingCount++
		}
	}
	v.dispatchedMu.RUnlock()

	// 1. ProcessingCount 먼저 분배
	processingAvailable := v.config.ProcessingCount - processingCount
	if processingAvailable > 0 {
		processingTasks := v.allocateTasks(processingAvailable, "Processing")
		v.dispatchTasks(processingTasks, "Processing")
	}

	// 2. PendingCount 분배
	pendingAvailable := v.config.PendingCount - pendingCount
	if pendingAvailable > 0 {
		pendingTasks := v.allocateTasks(pendingAvailable, "Pending")
		v.dispatchTasks(pendingTasks, "Pending")
	}

	return nil
}

func (v *Scheduler) allocateTasks(available int, status string) []*Task {
	v.statsMu.RLock()
	userCount := len(v.userStats)
	v.statsMu.RUnlock()

	if userCount == 0 || available <= 0 {
		return []*Task{}
	}

	// 유저별 pending task 개수 조회 (pending만)
	v.statsMu.RLock()
	allUsers := make([]*UserStat, 0, len(v.userStats))
	for _, stat := range v.userStats {
		if stat.PendingCount > 0 {
			allUsers = append(allUsers, stat)
		}
	}
	v.statsMu.RUnlock()

	if len(allUsers) == 0 {
		return []*Task{}
	}

	// PendingCount 기준 내림차순 정렬 (많은 순서)
	sort.Slice(allUsers, func(i, j int) bool {
		return allUsers[i].PendingCount > allUsers[j].PendingCount
	})

	userCount = len(allUsers)
	maxDedicated := v.config.MaxDedicatedUsers
	tasks := make([]*Task, 0, available)

	// Case 1: 유저 수 <= MaxDedicatedUsers (모두 동일하게 분배)
	if userCount <= maxDedicated {
		perUser := available / userCount
		remainder := available % userCount

		for i, user := range allUsers {
			quota := perUser
			if i < remainder {
				quota++
			}
			if quota > 0 {
				userTasks := v.fetchUserPendingTasksFIFO(user.UserID, quota)
				tasks = append(tasks, userTasks...)
			}
		}
		log.Debugf("[%s] %d users (≤ %d), equal distribution: %d per user",
			status, userCount, maxDedicated, perUser)
		return tasks
	}

	// Case 2: 유저 수 > MaxDedicatedUsers
	// Dedicated 영역: 상위 MaxDedicatedUsers명에게 DedicatedQuotaPercent만큼 할당
	dedicatedQuota := int(float64(available)*v.config.DedicatedQuotaPercent + 0.5) // 반올림
	if dedicatedQuota < maxDedicated {
		dedicatedQuota = maxDedicated // 최소한 1개씩은 보장
	}
	perDedicated := dedicatedQuota / maxDedicated

	dedicatedUsers := allUsers[:maxDedicated]
	sharedUsers := allUsers[maxDedicated:]

	// Dedicated 유저들에게 할당 (FIFO 순서로 발행)
	dedicatedAllocated := 0
	for _, user := range dedicatedUsers {
		if perDedicated > 0 {
			userTasks := v.fetchUserPendingTasksFIFO(user.UserID, perDedicated)
			tasks = append(tasks, userTasks...)
			dedicatedAllocated += len(userTasks)
		}
	}

	// Shared 영역: 남은 슬롯 계산
	sharedQuota := available - dedicatedAllocated

	// Shared 유저들을 요청 적은 순으로 정렬
	sort.Slice(sharedUsers, func(i, j int) bool {
		return sharedUsers[i].PendingCount < sharedUsers[j].PendingCount
	})

	// Shared 유저들에게 요청 적은 순으로 round-robin 방식 할당
	if sharedQuota > 0 && len(sharedUsers) > 0 {
		perShared := sharedQuota / len(sharedUsers)
		remainder := sharedQuota % len(sharedUsers)

		for i, user := range sharedUsers {
			quota := perShared
			if i < remainder {
				quota++
			}
			if quota > 0 {
				userTasks := v.fetchUserPendingTasksFIFO(user.UserID, quota)
				tasks = append(tasks, userTasks...)
			}
		}
	}

	log.Debugf("[%s] %d users (> %d MaxDedicated): dedicated=%d users (quota=%d each), shared=%d users (quota=%d total)",
		status, userCount, maxDedicated, maxDedicated, perDedicated, len(sharedUsers), sharedQuota)

	// Dedicated <-> Shared 교체 체크
	if len(sharedUsers) > 0 && len(dedicatedUsers) > 0 {
		// Shared에서 가장 많은 유저 (정렬 후 마지막)
		largestShared := sharedUsers[len(sharedUsers)-1]
		// Dedicated에서 가장 적은 유저 (정렬 시 마지막)
		smallestDedicated := dedicatedUsers[len(dedicatedUsers)-1]

		if largestShared.PendingCount > smallestDedicated.PendingCount {
			log.Infof("[%s] 🔄 Swap candidate: shared[%s]=%d > dedicated[%s]=%d (will swap in next cycle)",
				status, largestShared.UserID, largestShared.PendingCount,
				smallestDedicated.UserID, smallestDedicated.PendingCount)
			// 실제 교체는 다음 통계 갱신 시 자동으로 반영됨 (정렬 기준이 PendingCount이므로)
		}
	}

	return tasks
}

// fetchUserPendingTasksFIFO : 특정 유저의 pending task를 FIFO 순서로 가져옴
func (v *Scheduler) fetchUserPendingTasksFIFO(userID string, limit int) []*Task {
	v.tasksMu.RLock()
	defer v.tasksMu.RUnlock()

	tasks := make([]*Task, 0, limit)
	// 단순히 넣은 순서대로 발행 (map iteration order는 랜덤이지만, 테스트용으로 충분)
	for _, task := range v.tasks {
		if task.UserID == userID && task.Status == "pending" {
			tasks = append(tasks, task)
			if len(tasks) >= limit {
				break
			}
		}
	}
	return tasks
}

// startStatRefresher : 통계 갱신용
func (v *Scheduler) startStatRefresher(ctx context.Context) {
	ticker := time.NewTicker(v.config.StatRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.refreshStats()
			v.recalculateQuotas() // 할당량 재계산
		}
	}
}

func (v *Scheduler) recalculateQuotas() {
	// 이제는 필요 없지만 나중을 위해 유지
	// allocateTasks에서 직접 계산하므로 여기서는 통계만 갱신
	log.Debugf("Quota recalculation triggered (stats refreshed)")
}

// refreshStats : 통계 조회 (POC : in-memory)
func (v *Scheduler) refreshStats() {
	v.tasksMu.RLock()
	defer v.tasksMu.RUnlock()

	userCounts := make(map[string]*UserStat)

	for _, task := range v.tasks {
		if userCounts[task.UserID] == nil {
			userCounts[task.UserID] = &UserStat{
				UserID:      task.UserID,
				LastUpdated: time.Now(),
			}
		}

		switch task.Status {
		case "pending":
			userCounts[task.UserID].PendingCount++
		case "Processing", "Pending":
			userCounts[task.UserID].RunningCount++
		}
	}

	v.statsMu.Lock()
	v.userStats = userCounts
	v.statsMu.Unlock()
}

// ========================
// TODO : replace RDS
func (v *Scheduler) fetchUserPendingTasks(userID string, limit int) []*Task {
	v.tasksMu.RLock()
	defer v.tasksMu.RUnlock()

	tasks := make([]*Task, 0, limit)
	for _, task := range v.tasks {
		if task.UserID == userID && task.Status == "pending" {
			tasks = append(tasks, task)
			if len(tasks) >= limit {
				break
			}
		}
	}
	return tasks
}

func (v *Scheduler) fetchAllPendingTasks() []*Task {
	v.tasksMu.RLock()
	defer v.tasksMu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range v.tasks {
		if task.Status == "pending" {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (v *Scheduler) dispatchTasks(tasks []*Task, status string) error {
	if len(tasks) == 0 {
		return nil
	}

	log.Printf("[%s] dispatching %d tasks", status, len(tasks))

	// dispatchedTasks에 추가
	v.dispatchedMu.Lock()
	for _, task := range tasks {
		task.Status = status
		v.dispatchedTasks[task.ID] = task
		log.Printf("  - task=%s user=%s status=%s", task.ID, task.UserID, status)
	}
	v.dispatchedMu.Unlock()

	// 원본 tasks에서도 상태 업데이트
	v.tasksMu.Lock()
	for _, task := range tasks {
		if originalTask, exists := v.tasks[task.ID]; exists {
			originalTask.Status = status
		}
	}
	v.tasksMu.Unlock()

	return nil
}

// getExternalQueueCount : TODO : rabbitmq
func (v *Scheduler) getExternalQueueCount() int {
	// 실제로는 RabbitMQ message count 조회
	// 분배 보고 싶으니까 0으로 해야징 :)
	return 0
}

// ★ startWorkerSimulation : Worker 완료 시뮬레이션 (POC용)
func (v *Scheduler) startWorkerSimulation(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second) // 2초마다 일부 task 완료 처리
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.simulateTaskCompletion()
		}
	}
}

// simulateTaskCompletion : 일부 task를 완료 처리하여 dispatchedTasks에서 제거
func (v *Scheduler) simulateTaskCompletion() {
	v.dispatchedMu.Lock()
	defer v.dispatchedMu.Unlock()

	// ProcessingCount 개수만큼 동시 처리 가능하다고 가정
	// 2초마다 일부(예: ProcessingCount의 1/4) 완료
	completeCount := v.config.ProcessingCount / 4
	if completeCount == 0 {
		completeCount = 1
	}

	completed := 0
	for taskID, task := range v.dispatchedTasks {
		if completed >= completeCount {
			break
		}

		// dispatchedTasks에서 제거
		delete(v.dispatchedTasks, taskID)

		// 원본 tasks에서도 완료 처리
		v.tasksMu.Lock()
		if originalTask, exists := v.tasks[taskID]; exists {
			originalTask.Status = "completed"
		}
		v.tasksMu.Unlock()

		log.Debugf("✓ Worker completed: task=%s user=%s", task.ID, task.UserID)
		completed++
	}

	if completed > 0 {
		log.Debugf("✓ Worker simulation: completed %d tasks, remaining in queue: %d",
			completed, len(v.dispatchedTasks))
	}
}

// ========== 테스트용 헬퍼 ==========

func (v *Scheduler) AddTask(task *Task) {
	v.tasksMu.Lock()
	defer v.tasksMu.Unlock()
	v.tasks[task.ID] = task
}
