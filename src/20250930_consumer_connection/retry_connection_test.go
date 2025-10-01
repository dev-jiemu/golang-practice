package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRabbitMQURL = "amqp://guest:guest@localhost:5672/"
	testQueueName   = "jiemu-worker"
)

// 테스트 헬퍼: 큐 정리
// testing.TB는 *testing.T와 *testing.B 모두 호환되는 인터페이스
func cleanupQueue(tb testing.TB, queueName string) {
	conn, err := amqp.Dial(testRabbitMQURL)
	require.NoError(tb, err)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(tb, err)
	defer ch.Close()

	_, err = ch.QueueDelete(queueName, false, false, false)
	if err != nil {
		tb.Logf("Queue delete failed (may not exist): %v", err)
	}
}

// 테스트 헬퍼: 메시지 발행
func publishMessages(tb testing.TB, queueName string, count int) {
	conn, err := amqp.Dial(testRabbitMQURL)
	require.NoError(tb, err)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(tb, err)
	defer ch.Close()

	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	require.NoError(tb, err)

	for i := 0; i < count; i++ {
		body := fmt.Sprintf("test message %d", i)
		err = ch.Publish(
			"",        // exchange
			queueName, // routing key
			false,     // mandatory
			false,     // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
				MessageId:   fmt.Sprintf("msg-%d", i),
			},
		)
		require.NoError(tb, err)
	}
}

// Test 1: 기본 메시지 처리
func TestBasicMessageConsumption(t *testing.T) {
	queueName := testQueueName + "-basic"
	cleanupQueue(t, queueName)
	defer cleanupQueue(t, queueName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 메시지 발행
	publishMessages(t, queueName, 5)

	// Consumer 시작
	consumer := NewRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 2, 10*time.Second)
	consumer.ProcessFunc = func(ctx context.Context, message amqp.Delivery) error {
		slog.Info("Processing test message", "body", string(message.Body))
		time.Sleep(100 * time.Millisecond) // 처리 시뮬레이션
		return nil
	}

	go consumer.Start()

	// 모든 메시지가 처리될 때까지 대기
	assert.Eventually(t, func() bool {
		return consumer.ProcessCount.Load() == 5
	}, 10*time.Second, 100*time.Millisecond, "Should process all 5 messages")

	cancel()
	<-consumer.Done()
}

// Test 2: 에러 처리 및 재큐잉
func TestMessageRequeue(t *testing.T) {
	queueName := testQueueName + "-requeue"
	cleanupQueue(t, queueName)
	defer cleanupQueue(t, queueName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	publishMessages(t, queueName, 1)

	consumer := NewRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 1, 10*time.Second)

	var attemptCount atomic.Int32
	consumer.ProcessFunc = func(ctx context.Context, message amqp.Delivery) error {
		count := attemptCount.Add(1)
		if count == 1 {
			// 첫 번째 시도는 실패
			return fmt.Errorf("simulated processing error")
		}
		// 두 번째 시도는 성공
		return nil
	}

	go consumer.Start()

	// 재시도로 인해 2번 처리되어야 함
	assert.Eventually(t, func() bool {
		return consumer.ProcessCount.Load() >= 2
	}, 15*time.Second, 100*time.Millisecond, "Should retry failed message")

	cancel()
	<-consumer.Done()
}

// Test 3: Context 취소 시 정상 종료
func TestGracefulShutdown(t *testing.T) {
	queueName := testQueueName + "-shutdown"
	cleanupQueue(t, queueName)
	defer cleanupQueue(t, queueName)

	ctx, cancel := context.WithCancel(context.Background())

	publishMessages(t, queueName, 10)

	consumer := NewRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 2, 10*time.Second)
	consumer.ProcessFunc = func(ctx context.Context, message amqp.Delivery) error {
		time.Sleep(500 * time.Millisecond) // 느린 처리 시뮬레이션
		return nil
	}

	go consumer.Start()

	// 일부 메시지 처리 후 취소
	time.Sleep(1 * time.Second)
	cancel()

	// Consumer가 정상 종료되는지 확인
	select {
	case <-consumer.Done():
		t.Log("Consumer shut down gracefully")
	case <-time.After(10 * time.Second):
		t.Fatal("Consumer did not shut down in time")
	}

	processedCount := consumer.ProcessCount.Load()
	t.Logf("Processed %d messages before shutdown", processedCount)
	assert.Greater(t, processedCount, int32(0), "Should process at least some messages")
}

// Test 4: 타임아웃 처리
func TestMessageTimeout(t *testing.T) {
	queueName := testQueueName + "-timeout"
	cleanupQueue(t, queueName)
	defer cleanupQueue(t, queueName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	publishMessages(t, queueName, 1)

	consumer := NewRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 1, 2*time.Second)
	consumer.ProcessFunc = func(ctx context.Context, message amqp.Delivery) error {
		// 타임아웃보다 긴 처리 시간
		select {
		case <-time.After(5 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	go consumer.Start()

	// 타임아웃으로 실패하고 재시도되어야 함
	assert.Eventually(t, func() bool {
		return consumer.ProcessCount.Load() >= 2
	}, 15*time.Second, 100*time.Millisecond, "Should retry after timeout")

	cancel()
	<-consumer.Done()
}

// Test 5: 동시 워커 처리
func TestConcurrentWorkers(t *testing.T) {
	queueName := testQueueName + "-concurrent"
	cleanupQueue(t, queueName)
	defer cleanupQueue(t, queueName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messageCount := 10
	publishMessages(t, queueName, messageCount)

	var processingCount atomic.Int32
	var maxConcurrent atomic.Int32

	consumer := NewRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 3, 10*time.Second)
	consumer.ProcessFunc = func(ctx context.Context, message amqp.Delivery) error {
		current := processingCount.Add(1)

		// 최대 동시 처리 수 기록
		for {
			max := maxConcurrent.Load()
			if current <= max || maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}

		time.Sleep(500 * time.Millisecond)
		processingCount.Add(-1)
		return nil
	}

	go consumer.Start()

	assert.Eventually(t, func() bool {
		return consumer.ProcessCount.Load() == int32(messageCount)
	}, 20*time.Second, 100*time.Millisecond, "Should process all messages")

	maxConcurrentValue := maxConcurrent.Load()
	t.Logf("Maximum concurrent processing: %d", maxConcurrentValue)
	assert.LessOrEqual(t, maxConcurrentValue, int32(3), "Should not exceed prefetch count")
	assert.Greater(t, maxConcurrentValue, int32(1), "Should process messages concurrently")

	cancel()
	<-consumer.Done()
}

// Benchmark: 처리 성능 측정
func BenchmarkMessageProcessing(b *testing.B) {
	queueName := testQueueName + "-bench"
	cleanupQueue(b, queueName)
	defer cleanupQueue(b, queueName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer := NewRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 5, 30*time.Second)
	consumer.ProcessFunc = func(ctx context.Context, message amqp.Delivery) error {
		// 최소한의 처리
		return nil
	}

	go consumer.Start()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		publishMessages(b, queueName, 1)
	}

	// 모든 메시지 처리 대기
	for consumer.ProcessCount.Load() < int32(b.N) {
		time.Sleep(10 * time.Millisecond)
	}

	b.StopTimer()
	cancel()
	<-consumer.Done()
}
