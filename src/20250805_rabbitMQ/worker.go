package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/streadway/amqp"
)

func main() {
	// RabbitMQ 비동기 처리 관련 worker with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer ch.Close()

	// durable : queue를 디스크에 저장함 => RabbitMQ 가 재시작되어도 메세지 유지하는거임
	q, err := ch.QueueDeclare("jiemu-worker", true, false, false, false, nil)
	if err != nil {
		log.Fatalln(err)
		return
	}

	// QoS : 한번에 하나의 메세지만 처리하게 하기
	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		log.Fatalln(err)
		return
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalln(err)
		return
	}

	go func() {
		for {
			select {
			case d := <-msgs:
				fmt.Printf(" [x] Received %s\n", d.Body)

				// 작업 시뮬레이션 (점의 개수만큼 초 대기)
				dotCount := bytes.Count(d.Body, []byte("."))
				t := time.Duration(dotCount)
				time.Sleep(t * time.Second)

				fmt.Println(" [x] Done")

				// 작업 완료 후 수동으로 ACK
				d.Ack(false)
			case <-ctx.Done():
				fmt.Println("Consumer shutting down...")
				return
			}
		}
	}()

	<-sigCh
	fmt.Println("Shutdown signal received")
	cancel()

}
