package main

import (
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672")
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

	// QoS 설정: prefetch count = 3
	err = ch.Qos(3, 0, false)
	if err != nil {
		log.Fatalln("Failed to set QoS:", err)
		return
	}

	q, err := ch.QueueDeclare("jiemu-worker", true, false, false, false, nil)
	if err != nil {
		log.Fatalln(err)
		return
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (false로 설정하여 수동 Ack)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		log.Fatalln(err)
		return
	}

	consumeChannel := make(chan bool)

	// 이렇게 설정하면 QoS 를 3으로 줘도 사실상 하나씩 처리하는 거임
	/*
		go func() {
			for d := range msgs {
				log.Printf("Received a message: %s", d.Body)

				// 3초 대기
				time.Sleep(3 * time.Second)

				// 처리 완료 후 Ack
				d.Ack(false)

				log.Printf("Message processed and acked: %s", d.Body)
			}
		}()
	*/

	const workerCount = 3

	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			for d := range msgs {
				log.Printf("[Worker %d] Received a message: %s", workerID, d.Body)

				// 3초 대기 (실제 처리 시뮬레이션)
				time.Sleep(3 * time.Second)

				// 처리 완료 후 Ack
				d.Ack(false)

				log.Printf("[Worker %d] Message processed and acked: %s", workerID, d.Body)
			}
		}(i)
	}

	fmt.Println(" [*] Waiting for messages. To exit press CTRL+C")
	fmt.Println(" [*] QoS prefetch count: 3")
	fmt.Println(" [*] Processing delay: 3 seconds per message")
	<-consumeChannel
}
