package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ auto-reconnect pattern
// Ref. https://github.com/rabbitmq/amqp091-go/blob/main/reconnect_test.go

type RabbitMqConsumer struct {
	baseCtx    context.Context
	amqpURL    string
	queueName  string
	prefetch   int
	jobTimeout time.Duration
	done       chan struct{}
}

func NewRabbitMqConsumer(ctx context.Context, url, queueName string, prefetch int, jobTimeout time.Duration) *RabbitMqConsumer {
	return &RabbitMqConsumer{
		baseCtx:    ctx,
		amqpURL:    url,
		queueName:  queueName,
		prefetch:   prefetch,
		jobTimeout: jobTimeout,
		done:       make(chan struct{}),
	}
}

func (v *RabbitMqConsumer) Done() <-chan struct{} {
	return v.done
}

// setup: 연결 + 채널 + 큐 선언을 한 번에 처리
func (v *RabbitMqConsumer) setup() (*amqp.Connection, *amqp.Channel, error) {
	// TCP KeepAlive 설정
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Heartbeat 설정으로 연결
	conn, err := amqp.DialConfig(v.amqpURL, amqp.Config{
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

	// 큐 선언 (재연결 시마다 다시 선언)
	if _, err := channel.QueueDeclare(
		v.queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	); err != nil {
		channel.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	// QoS 설정 (재연결 시마다 다시 설정)
	if err := channel.Qos(v.prefetch, 0, false); err != nil {
		channel.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	return conn, channel, nil
}

// consume: setup + consumer 등록
func (v *RabbitMqConsumer) consume() (*amqp.Connection, <-chan amqp.Delivery, error) {
	conn, channel, err := v.setup()
	if err != nil {
		return nil, nil, err
	}

	deliveries, err := channel.ConsumeWithContext(
		v.baseCtx,
		v.queueName,
		"",    // consumer tag
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("failed to register a consumer: %w", err)
	}

	return conn, deliveries, nil
}

// Start: Auto-reconnect + Worker Pool 패턴
func (v *RabbitMqConsumer) Start() {
	defer close(v.done)

	mqLog := slog.With("mq", v.queueName)

	reconnectDelay := 5 * time.Second
	maxReconnectDelay := 5 * time.Minute

	for {
		// Context 취소 확인
		select {
		case <-v.baseCtx.Done():
			mqLog.Info("Base context cancelled, exiting consumer")
			return
		default:
		}

		mqLog.Info("Connecting to RabbitMQ...", "queue", v.queueName)

		// 연결 시도
		conn, deliveries, err := v.consume()
		if err != nil {
			mqLog.Error("Failed to connect", "error", err, "retry_after", reconnectDelay)
			time.Sleep(reconnectDelay)

			// Exponential backoff
			reconnectDelay *= 2
			if reconnectDelay > maxReconnectDelay {
				reconnectDelay = maxReconnectDelay
			}
			continue
		}

		// 연결 성공 시 딜레이 초기화
		reconnectDelay = 5 * time.Second
		mqLog.Info("Successfully connected to RabbitMQ", "queue", v.queueName)

		// 워커 풀로 메시지 처리
		// deliveries 채널이 닫히면 모든 워커가 종료되고 여기로 돌아옴
		v.runWorkerPool(deliveries, mqLog)

		// 연결 정리
		conn.Close()
		mqLog.Warn("Deliveries channel closed, reconnecting...", "delay", reconnectDelay)
		time.Sleep(reconnectDelay)
	}
}

// runWorkerPool: 워커 풀 실행 (기존 Start 로직과 동일)
func (v *RabbitMqConsumer) runWorkerPool(deliveries <-chan amqp.Delivery, mqLog *slog.Logger) {
	ctx, cancel := context.WithCancel(v.baseCtx)
	defer cancel()

	var wg sync.WaitGroup
	workerCount := v.prefetch // prefetch count와 동일하게 워커 생성

	// 워커 시작
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workId int) {
			defer wg.Done()

			mqLog.Debug("worker started", "work_id", workId)

			for {
				select {
				case message, ok := <-deliveries:
					if !ok {
						// deliveries 채널 닫힘 = 연결 끊김
						mqLog.Debug("deliveries channel closed, worker stopping", "work_id", workId)
						return
					}

					// 메시지 처리 (20분 걸려도 OK - 다른 워커들이 계속 처리함)
					if shouldStop := v.handleMessage(message, mqLog); shouldStop {
						mqLog.Debug("stop consuming due to shutdown or handling result", "work_id", workId)
						return
					}

				case <-ctx.Done():
					// Context 취소됨
					mqLog.Debug("worker stopping due to context cancellation", "work_id", workId)
					return
				}
			}
		}(i)
	}

	// Base context 모니터링
	go func() {
		<-v.baseCtx.Done()
		mqLog.Debug("base context cancelled")
		cancel()
	}()

	// 모든 워커 종료 대기
	// deliveries가 닫히면 모든 워커가 종료되고 여기서 리턴
	wg.Wait()
	mqLog.Info("All workers stopped")
}

// handleMessage: 개별 메시지 처리
func (v *RabbitMqConsumer) handleMessage(message amqp.Delivery, mqLog *slog.Logger) bool {
	// 타임아웃 컨텍스트 생성
	ctx, cancel := context.WithTimeout(v.baseCtx, v.jobTimeout)
	defer cancel()

	reqLog := mqLog.With(
		"message_id", message.MessageId,
		"delivery_tag", message.DeliveryTag,
	)

	reqLog.Info("Processing message", "body_length", len(message.Body))

	// 실제 메시지 처리
	if err := v.processBusinessLogic(ctx, message); err != nil {
		reqLog.Error("Failed to process message", "error", err)

		// 처리 실패 시 재큐잉
		if nackErr := message.Nack(false, true); nackErr != nil {
			reqLog.Error("Failed to nack message", "error", nackErr)
		}
		return false
	}

	// 처리 성공 시 Ack
	if err := message.Ack(false); err != nil {
		reqLog.Error("Failed to ack message", "error", err)
	} else {
		reqLog.Debug("message ack success")
	}

	return false
}

// processBusinessLogic: 실제 비즈니스 로직 처리
func (v *RabbitMqConsumer) processBusinessLogic(ctx context.Context, message amqp.Delivery) error {
	// TODO: 실제 비즈니스 로직 구현
	// 20분까지 걸릴 수 있는 긴 작업

	select {
	case <-ctx.Done():
		return fmt.Errorf("processing timeout or context cancelled")
	case <-time.After(time.Second * 2):
		// 실제 작업 시뮬레이션
		slog.Info("Business logic completed", "body", string(message.Body))
		return nil
	}
}

func exitSignal() <-chan os.Signal {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}

func main() {
	// Context 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Consumer 생성
	mqConsumer := NewRabbitMqConsumer(
		ctx,
		"amqp://guest:guest@localhost:5672/",
		"jiemu-worker",
		3,                               // prefetch count (워커 수)
		time.Duration(3600)*time.Second, // job timeout (1시간)
	)

	// Consumer 시작 (블로킹 모드, 고루틴으로 실행)
	slog.Info("Starting RabbitMQ consumer with auto-reconnect")
	go mqConsumer.Start()

	// 종료 신호 대기
	select {
	case <-exitSignal():
		slog.Info("Shutdown signal received")
	case <-mqConsumer.Done():
		slog.Info("Consumer exited normally")
	}

	// 종료 처리
	cancel()

	// Consumer 종료 대기
	<-mqConsumer.Done()
	slog.Info("Consumer stopped successfully, exiting")
}
