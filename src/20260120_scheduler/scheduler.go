package main

import (
	"context"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Task struct {
	ID        string
	UserID    string
	Status    string
	Priority  int
	CreatedAt time.Time
}

type UserStat struct {
	UserID       string
	PendingCount int
	RunningCount int
	FirstSeenAt  time.Time // 처음 등장한 시간 (FIFO 기준)
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
		config:     config,
		tasks:      make(map[string]*Task),
		userStats:  make(map[string]*UserStat),
		userQuotas: make(map[string]*UserQuota),
	}
}

func (v *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 통계 갱신용 고루틴
	go v.startStatRefresher(ctx)

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
	queueCount := v.getExternalQueueCount() // TODO : 실제로는 RabbitMQ 조회 해야함
	available := v.config.MaxSchedule - queueCount

	if available <= 0 {
		log.Printf("queue full: %d/%d", queueCount, v.config.MaxSchedule)
		return nil
	}

	v.quotaMu.RLock()
	hasQuota := len(v.userQuotas) > 0
	v.quotaMu.RUnlock()

	var allTasks []*Task
	if hasQuota {
		allTasks = v.processWithQuotas(available) // 할당량이 있으면 quota 기반 분배
	} else {
		allTasks = v.processSharedOnly(available)
	}

	return v.dispatchTasks(allTasks)
}

func (v *Scheduler) processWithQuotas(available int) []*Task {
	v.quotaMu.RLock()
	quotaUsers := make(map[string]*UserQuota)
	for key, value := range v.userQuotas {
		quotaUsers[key] = value
	}
	sharedQuota := v.quotaForShared
	v.quotaMu.RUnlock()

	tasks := make([]*Task, 0, available)

	for userID, quota := range quotaUsers {
		userLimit := min(quota.MaxSlots, available-len(tasks))
		if userLimit <= 0 {
			continue
		}

		userTasks := v.fetchUserPendingTasks(userID, userLimit)
		tasks = append(tasks, userTasks...)
	}

	if sharedQuota > 0 {
		remaining := min(sharedQuota, available-len(tasks))
		if remaining > 0 {
			log.Debugf("shared quota remaining: %d", remaining)

			sharedTasks := v.fetchSharedTasks(remaining)
			tasks = append(tasks, sharedTasks...)

			log.Debugf("[shared] allocated: %d tasks", len(tasks))
		}
	}

	return tasks
}

func (v *Scheduler) processSharedOnly(available int) []*Task {
	log.Debugf("  no heavy users, processing all as shared")
	allTasks := v.fetchAllPendingTasks()

	v.calculatePriority(allTasks)
	sort.Slice(allTasks, func(i, j int) bool {
		return allTasks[i].Priority > allTasks[j].Priority
	})

	if len(allTasks) > available {
		allTasks = allTasks[:available]
	}

	return allTasks
}

// fetchSharedTasks : quota 없는 유저들의 task (aging 순)
func (v *Scheduler) fetchSharedTasks(limit int) []*Task {
	v.quotaMu.RLock()
	quotaUserIDs := make(map[string]bool)
	for userID := range v.userQuotas {
		quotaUserIDs[userID] = true
	}
	v.quotaMu.RUnlock()

	allTasks := v.fetchAllPendingTasks()
	sharedTasks := make([]*Task, 0)

	for _, task := range allTasks {
		if !quotaUserIDs[task.UserID] {
			sharedTasks = append(sharedTasks, task)
		}
	}

	v.calculatePriority(sharedTasks)
	sort.Slice(sharedTasks, func(i, j int) bool {
		return sharedTasks[i].Priority > sharedTasks[j].Priority
	})

	if len(sharedTasks) > limit {
		sharedTasks = sharedTasks[:limit]
	}

	return sharedTasks
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
	v.statsMu.RLock()

	allUsers := make([]*UserStat, 0)
	for _, stat := range v.userStats {
		if stat.PendingCount > 0 {
			allUsers = append(allUsers, stat)
		}
	}
	v.statsMu.RUnlock()

	// ★ 중요: 먼저 들어온 순서대로 정렬 (FIFO)
	sort.Slice(allUsers, func(i, j int) bool {
		return allUsers[i].FirstSeenAt.Before(allUsers[j].FirstSeenAt)
	})

	available := v.config.MaxSchedule
	newQuotas := make(map[string]*UserQuota)

	userCount := len(allUsers)

	if userCount == 0 {
		v.quotaMu.Lock()
		v.userQuotas = newQuotas
		v.quotaMu.Unlock()
		return
	}

	dedicatedUserCount := min(userCount, v.config.MaxDedicatedUsers)
	sharedUserCount := userCount - dedicatedUserCount

	var quotaPerDedicated int
	var quotaForShared int

	if sharedUserCount > 0 {
		totalWorkers := dedicatedUserCount + 1
		quotaPerWorker := available / totalWorkers
		quotaPerDedicated = quotaPerWorker
		quotaForShared = quotaPerWorker

		log.Debugf("✓ Quota calculation: %d users (%d dedicated FIFO, %d shared), quota=%d each", userCount, dedicatedUserCount, sharedUserCount, quotaPerWorker)
	} else {
		quotaPerDedicated = available / dedicatedUserCount
		quotaForShared = 0

		log.Debugf("✓ Quota calculation: %d users (all dedicated FIFO), quota=%d each", userCount, quotaPerDedicated)
	}

	// ★ 먼저 온 순서대로 상위 N명에게 할당
	for i := 0; i < dedicatedUserCount && i < len(allUsers); i++ {
		user := allUsers[i]
		newQuotas[user.UserID] = &UserQuota{
			UserID:      user.UserID,
			MaxSlots:    quotaPerDedicated,
			LastUpdated: time.Now(),
		}
	}

	v.quotaMu.Lock()
	v.userQuotas = newQuotas
	v.quotaForShared = quotaForShared
	v.quotaMu.Unlock()

}

// refreshStats : 통계 조회 (POC : in-memory)
func (v *Scheduler) refreshStats() {
	v.tasksMu.RLock()
	defer v.tasksMu.RUnlock()

	userCounts := make(map[string]*UserStat)

	for _, task := range v.tasks {
		if userCounts[task.UserID] == nil {
			// ★ 기존 stat이 있으면 FirstSeenAt 유지 (FIFO 추적)
			v.statsMu.RLock()
			existingStat := v.userStats[task.UserID]
			v.statsMu.RUnlock()

			firstSeen := time.Now()
			if existingStat != nil {
				firstSeen = existingStat.FirstSeenAt
			}

			userCounts[task.UserID] = &UserStat{
				UserID:      task.UserID,
				FirstSeenAt: firstSeen,
				LastUpdated: time.Now(),
			}
		}

		switch task.Status {
		case "pending":
			userCounts[task.UserID].PendingCount++
		case "running", "queued":
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

func (v *Scheduler) calculatePriority(tasks []*Task) {
	now := time.Now()
	for _, task := range tasks {
		// Aging: 오래 기다린 task일수록 높은 우선순위
		waitSeconds := int(now.Sub(task.CreatedAt).Seconds())
		task.Priority = waitSeconds
	}
}

func (v *Scheduler) dispatchTasks(tasks []*Task) error {
	// 실제로는 RabbitMQ publish
	log.Printf("dispatching %d tasks to queue", len(tasks))

	for _, task := range tasks {
		task.Status = "queued"
		log.Printf("  - task=%s user=%s", task.ID, task.UserID)
	}

	return nil
}

// getExternalQueueCount : TODO : rabbitmq
func (v *Scheduler) getExternalQueueCount() int {
	// 실제로는 RabbitMQ message count 조회
	// 분배 보고 싶으니까 0으로 해야징 :)
	return 0
}

// ========== 테스트용 헬퍼 ==========

func (v *Scheduler) AddTask(task *Task) {
	v.tasksMu.Lock()
	defer v.tasksMu.Unlock()
	v.tasks[task.ID] = task
}
