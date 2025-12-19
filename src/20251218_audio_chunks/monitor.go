package main

import (
	"context"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ResourceMonitor : 시스템 리소스 모니터링
type ResourceMonitor struct {
	mu                sync.Mutex
	startTime         time.Time
	activeWorkers     int
	maxActiveWorkers  int
	totalChunks       int
	processedChunks   int
	failedChunks      int
	cpuSamples        []float64
	memorySamplesMB   []float64
	workerStartTimes  map[int]time.Time
	workerDurations   []time.Duration
	chunkProcessTimes []time.Duration
}

func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		startTime:         time.Now(),
		cpuSamples:        make([]float64, 0),
		memorySamplesMB:   make([]float64, 0),
		workerStartTimes:  make(map[int]time.Time),
		workerDurations:   make([]time.Duration, 0),
		chunkProcessTimes: make([]time.Duration, 0),
	}
}

// StartMonitoring : 백그라운드에서 주기적으로 시스템 리소스 수집
func (m *ResourceMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.collectResourceStats()
			}
		}
	}()
}

// collectResourceStats : 현재 메모리 및 고루틴 수 수집
func (m *ResourceMonitor) collectResourceStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// 메모리 사용량 (MB)
	memoryMB := float64(mem.Alloc) / 1024 / 1024
	m.memorySamplesMB = append(m.memorySamplesMB, memoryMB)

	// 고루틴 수는 CPU 사용의 간접 지표
	numGoroutines := float64(runtime.NumGoroutine())
	m.cpuSamples = append(m.cpuSamples, numGoroutines)
}

// WorkerStart : 워커 시작 기록
func (m *ResourceMonitor) WorkerStart(workerID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeWorkers++
	if m.activeWorkers > m.maxActiveWorkers {
		m.maxActiveWorkers = m.activeWorkers
	}
	m.workerStartTimes[workerID] = time.Now()
}

// WorkerEnd : 워커 종료 기록
func (m *ResourceMonitor) WorkerEnd(workerID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeWorkers--
	if startTime, ok := m.workerStartTimes[workerID]; ok {
		duration := time.Since(startTime)
		m.workerDurations = append(m.workerDurations, duration)
		delete(m.workerStartTimes, workerID)
	}
}

// ChunkProcessed : 청크 처리 완료 기록
func (m *ResourceMonitor) ChunkProcessed(success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processedChunks++
	if !success {
		m.failedChunks++
	}
	m.chunkProcessTimes = append(m.chunkProcessTimes, duration)
}

// SetTotalChunks : 전체 청크 수 설정
func (m *ResourceMonitor) SetTotalChunks(total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalChunks = total
}

// PrintSummary : 최종 요약 정보 출력
func (m *ResourceMonitor) PrintSummary() {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalDuration := time.Since(m.startTime)

	log.Println(strings.Repeat("=", 60))
	log.Println("📊 Resource Usage Summary")
	log.Println(strings.Repeat("=", 60))

	// 시간 정보
	log.Printf("⏱️  Total Processing Time: %s\n", totalDuration.Round(time.Millisecond))

	// 청크 처리 정보
	log.Printf("📦 Total Chunks: %d\n", m.totalChunks)
	log.Printf("✅ Successfully Processed: %d\n", m.processedChunks-m.failedChunks)
	log.Printf("❌ Failed: %d\n", m.failedChunks)

	if m.totalChunks > 0 {
		successRate := float64(m.processedChunks-m.failedChunks) / float64(m.totalChunks) * 100
		log.Printf("📈 Success Rate: %.2f%%\n", successRate)
	}

	// 워커 정보
	log.Printf("👷 Max Concurrent Workers: %d\n", m.maxActiveWorkers)

	if len(m.chunkProcessTimes) > 0 {
		avgChunkTime := m.averageDuration(m.chunkProcessTimes)
		minChunkTime := m.minDuration(m.chunkProcessTimes)
		maxChunkTime := m.maxDuration(m.chunkProcessTimes)

		log.Printf("⏳ Avg Chunk Processing Time: %s\n", avgChunkTime.Round(time.Millisecond))
		log.Printf("   Min: %s, Max: %s\n", minChunkTime.Round(time.Millisecond), maxChunkTime.Round(time.Millisecond))
	}

	// 메모리 정보
	if len(m.memorySamplesMB) > 0 {
		avgMem := m.average(m.memorySamplesMB)
		maxMem := m.max(m.memorySamplesMB)
		minMem := m.min(m.memorySamplesMB)

		log.Printf("💾 Memory Usage (MB):\n")
		log.Printf("   Avg: %.2f MB, Min: %.2f MB, Max: %.2f MB\n", avgMem, minMem, maxMem)
	}

	// 고루틴 정보
	if len(m.cpuSamples) > 0 {
		avgGoroutines := m.average(m.cpuSamples)
		maxGoroutines := m.max(m.cpuSamples)

		log.Printf("🔄 Goroutines:\n")
		log.Printf("   Avg: %.0f, Max: %.0f\n", avgGoroutines, maxGoroutines)
	}

	// 처리율 계산
	if m.totalChunks > 0 && totalDuration.Seconds() > 0 {
		throughput := float64(m.totalChunks) / totalDuration.Seconds()
		log.Printf("⚡ Throughput: %.2f chunks/sec\n", throughput)
	}

	log.Println(strings.Repeat("=", 60))
}

// PrintProgress : 실시간 진행상황 출력
func (m *ResourceMonitor) PrintProgress() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.totalChunks == 0 {
		return
	}

	progress := float64(m.processedChunks) / float64(m.totalChunks) * 100

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	memoryMB := float64(mem.Alloc) / 1024 / 1024

	log.Printf("[Progress] %.1f%% (%d/%d) | Active Workers: %d | Memory: %.2f MB | Goroutines: %d",
		progress,
		m.processedChunks,
		m.totalChunks,
		m.activeWorkers,
		memoryMB,
		runtime.NumGoroutine(),
	)
}

// Helper functions
func (m *ResourceMonitor) average(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	return sum / float64(len(samples))
}

func (m *ResourceMonitor) min(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	min := samples[0]
	for _, v := range samples {
		if v < min {
			min = v
		}
	}
	return min
}

func (m *ResourceMonitor) max(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	max := samples[0]
	for _, v := range samples {
		if v > max {
			max = v
		}
	}
	return max
}

func (m *ResourceMonitor) averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sum := time.Duration(0)
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func (m *ResourceMonitor) minDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	min := durations[0]
	for _, d := range durations {
		if d < min {
			min = d
		}
	}
	return min
}

func (m *ResourceMonitor) maxDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	max := durations[0]
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	return max
}
