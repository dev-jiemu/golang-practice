package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRabbitMQURL = "amqp://guest:guest@localhost:5672/"
	testQueueName   = "test-queue"
)

// 테스트용 비즈니스 로직을 주입할 수 있도록 수정된 Consumer
type TestableRabbitMqConsumer struct {
	baseCtx      context.Context
	amqpURL      string
	queueName    string
	prefetch     int
	jobTimeout   time.Duration
	done         chan struct{}
	processFunc  func(ctx context.Context, message amqp.Delivery) error
	processCount atomic.Int32
}

func NewTestableRabbitMqConsumer(ctx context.Context, url, queueName string, prefetch int, jobTimeout time.Duration) *TestableRabbitMqConsumer {
	return &TestableRabbitMqConsumer{
		baseCtx:    ctx,
		amqpURL:    url,
		queueName:  queueName,
		prefetch:   prefetch,
		jobTimeout: jobTimeout,
		done:       make(chan struct{}),
	}
}

func (t *TestableRabbitMqConsumer) Done() <-chan struct{} {
	return t.done
}

// RabbitMqConsumer의 메서드들을 복사 (setup, consume, Start, runWorkerPool, handleMessage)
// 하지만 processBusinessLogic만 테스트용으로 오버라이드

func (t *TestableRabbitMqConsumer) setup() (*amqp.Connection, *amqp.Channel, error) {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := amqp.DialConfig(t.amqpURL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		Dial: func(network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	if _, err := channel.QueueDeclare(
		t.queueName,
		true, false, false, false, nil,
	); err != nil {
		channel.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	if err := channel.Qos(t.prefetch, 0, false); err != nil {
		channel.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return conn, channel, nil
}

func (t *TestableRabbitMqConsumer) consume() (*amqp.Connection, <-chan amqp.Delivery, error) {
	conn, channel, err := t.setup()
	if err != nil {
		return nil, nil, err
	}

	deliveries, err := channel.ConsumeWithContext(
		t.baseCtx,
		t.queueName,
		"", false, false, false, false, nil,
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to register a consumer: %w", err)
	}

	return conn, deliveries, nil
}

func (t *TestableRabbitMqConsumer) Start() {
	defer close(t.done)

	mqLog := slog.With("mq", t.queueName)
	reconnectDelay := 5 * time.Second
	maxReconnectDelay := 5 * time.Minute

	for {
		select {
		case <-t.baseCtx.Done():
			mqLog.Info("Base context cancelled, exiting consumer")
			return
		default:
		}

		mqLog.Info("Connecting to RabbitMQ...", "queue", t.queueName)

		conn, deliveries, err := t.consume()
		if err != nil {
			mqLog.Error("Failed to connect", "error", err, "retry_after", reconnectDelay)
			time.Sleep(reconnectDelay)
			reconnectDelay *= 2
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
			continue
		}

		reconnectDelay = 5 * time.Second
		mqLog.Info("Successfully connected to RabbitMQ", "queue", t.queueName)

		t.runWorkerPool(deliveries, mqLog)

		conn.Close()
		mqLog.Warn("Deliveries channel closed, reconnecting...", "delay", reconnectDelay)
		time.Sleep(reconnectDelay)
	}
}

func (t *TestableRabbitMqConsumer) runWorkerPool(deliveries <-chan amqp.Delivery, mqLog *slog.Logger) {
	ctx, cancel := context.WithCancel(t.baseCtx)
	defer cancel()

	var wg sync.WaitGroup
	workerCount := t.prefetch

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workId int) {
			defer wg.Done()
			mqLog.Debug("worker started", "work_id", workId)

			for {
				select {
				case message, ok := <-deliveries:
					if !ok {
						mqLog.Debug("deliveries channel closed, worker stopping", "work_id", workId)
						return
					}

					if shouldStop := t.handleMessage(message, mqLog); shouldStop {
						mqLog.Debug("stop consuming due to shutdown or handling result", "work_id", workId)
						return
					}

				case <-ctx.Done():
					mqLog.Debug("worker stopping due to context cancellation", "work_id", workId)
					return
				}
			}
		}(i)
	}

	go func() {
		<-t.baseCtx.Done()
		mqLog.Debug("base context cancelled")
		cancel()
	}()

	wg.Wait()
	mqLog.Info("All workers stopped")
}

func (t *TestableRabbitMqConsumer) handleMessage(message amqp.Delivery, mqLog *slog.Logger) bool {
	ctx, cancel := context.WithTimeout(t.baseCtx, t.jobTimeout)
	defer cancel()

	reqLog := mqLog.With(
		"message_id", message.MessageId,
		"delivery_tag", message.DeliveryTag,
	)

	reqLog.Info("Processing message", "body_length", len(message.Body))

	if err := t.processBusinessLogic(ctx, message); err != nil {
		reqLog.Error("Failed to process message", "error", err)

		if nackErr := message.Nack(false, true); nackErr != nil {
			reqLog.Error("Failed to nack message", "error", nackErr)
		}
		return false
	}

	if err := message.Ack(false); err != nil {
		reqLog.Error("Failed to ack message", "error", err)
	} else {
		reqLog.Debug("message ack success")
	}

	return false
}

// processBusinessLogic을 오버라이드하여 테스트용 함수 사용
func (t *TestableRabbitMqConsumer) processBusinessLogic(ctx context.Context, message amqp.Delivery) error {
	t.processCount.Add(1)
	if t.processFunc != nil {
		return t.processFunc(ctx, message)
	}
	return nil
}

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
	consumer := NewTestableRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 2, 10*time.Second)
	consumer.processFunc = func(ctx context.Context, message amqp.Delivery) error {
		slog.Info("Processing test message", "body", string(message.Body))
		time.Sleep(100 * time.Millisecond) // 처리 시뮬레이션
		return nil
	}

	go consumer.Start()

	// 모든 메시지가 처리될 때까지 대기
	assert.Eventually(t, func() bool {
		return consumer.processCount.Load() == 5
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

	consumer := NewTestableRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 1, 10*time.Second)

	var attemptCount atomic.Int32
	consumer.processFunc = func(ctx context.Context, message amqp.Delivery) error {
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
		return consumer.processCount.Load() >= 2
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

	consumer := NewTestableRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 2, 10*time.Second)
	consumer.processFunc = func(ctx context.Context, message amqp.Delivery) error {
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

	processedCount := consumer.processCount.Load()
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

	consumer := NewTestableRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 1, 2*time.Second)
	consumer.processFunc = func(ctx context.Context, message amqp.Delivery) error {
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
		return consumer.processCount.Load() >= 2
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

	consumer := NewTestableRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 3, 10*time.Second)
	consumer.processFunc = func(ctx context.Context, message amqp.Delivery) error {
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
		return consumer.processCount.Load() == int32(messageCount)
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

	consumer := NewTestableRabbitMqConsumer(ctx, testRabbitMQURL, queueName, 5, 30*time.Second)
	consumer.processFunc = func(ctx context.Context, message amqp.Delivery) error {
		// 최소한의 처리
		return nil
	}

	go consumer.Start()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		publishMessages(b, queueName, 1)
	}

	// 모든 메시지 처리 대기
	for consumer.processCount.Load() < int32(b.N) {
		time.Sleep(10 * time.Millisecond)
	}

	b.StopTimer()
	cancel()
	<-consumer.Done()
}
