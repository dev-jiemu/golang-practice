package main

import (
	"fmt"
	"sync"
	"time"
)

func barrierWithRetry() {
	fmt.Println("\n=== Whisper API Barrier with Retry ===")

	type WhisperJob struct {
		JobID     int
		FilePath  string
		StartTime float64 // 원본에서의 시작 시간 (초)
	}

	type JobResult struct {
		JobID     int
		Subtitles string
		StartTime float64
		Error     error
		Attempts  int
	}

	// 작업 목록 정의
	jobs := []WhisperJob{
		{JobID: 0, FilePath: "segment_0.webm", StartTime: 0.0},
		{JobID: 1, FilePath: "segment_1.webm", StartTime: 1200.0}, // 20분
		{JobID: 2, FilePath: "segment_2.webm", StartTime: 2400.0}, // 40분
	}

	jobCount := len(jobs)
	resultChan := make(chan JobResult, jobCount)
	var wg sync.WaitGroup

	// 각 sub job 실행 (재시도 로직 포함)
	for _, job := range jobs {
		wg.Add(1)
		go func(j WhisperJob) {
			defer wg.Done()

			maxRetries := 3
			var result JobResult

			for attempt := 1; attempt <= maxRetries; attempt++ {
				fmt.Printf("Job %d attempt %d/%d\n", j.JobID, attempt, maxRetries)

				// CHECK : call api
				subtitles := ""
				var err error

				if err == nil {
					result = JobResult{
						JobID:     j.JobID,
						Subtitles: subtitles,
						StartTime: j.StartTime,
						Error:     nil,
						Attempts:  attempt,
					}
					break
				}

				fmt.Printf("Job %d attempt %d failed: %v\n", j.JobID, attempt, err)

				if attempt == maxRetries {
					result = JobResult{
						JobID:     j.JobID,
						Error:     fmt.Errorf("job %d failed after %d attempts: %w", j.JobID, maxRetries, err),
						StartTime: j.StartTime,
						Attempts:  attempt,
					}
				} else {
					// 재시도 전 대기 (exponential backoff)
					backoffTime := time.Duration(attempt*attempt) * time.Second
					fmt.Printf("Job %d retrying in %v...\n", j.JobID, backoffTime)
					time.Sleep(backoffTime)
				}
			}

			resultChan <- result
		}(job)
	}

	// 별도 고루틴에서 WaitGroup 대기 후 채널 닫기
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 결과 수집
	results := make([]JobResult, jobCount)
	successCount := 0
	var failedJobs []int

	for result := range resultChan {
		results[result.JobID] = result

		if result.Error != nil {
			failedJobs = append(failedJobs, result.JobID)
			fmt.Printf("❌ Job %d permanently failed: %v\n", result.JobID, result.Error)
		} else {
			successCount++
			fmt.Printf("✅ Job %d succeeded after %d attempts\n", result.JobID, result.Attempts)
		}
	}

	// 배리어 지점: 모든 작업 완료 후 결과 분석
	fmt.Printf("\n=== Barrier Reached ===\n")
	fmt.Printf("Success: %d/%d jobs\n", successCount, jobCount)

	if len(failedJobs) > 0 {
		fmt.Printf("Failed jobs: %v\n", failedJobs)

		// 실패한 작업에 대한 정책 결정
		if successCount >= jobCount/2 { // 과반수 성공 시
			fmt.Println("⚠️  Partial success - proceeding with available data")
			// TODO : check
		} else {
			fmt.Println("❌ Too many failures - aborting main job")
			return
		}
	} else {
		fmt.Println("🎉 All jobs succeeded - starting main job")
		// NEXT : merge
	}
}
