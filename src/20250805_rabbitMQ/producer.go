package main

import (
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalln(err) // fail open channel
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare("jiemu-test", false, false, false, false, nil)
	if err != nil {
		log.Fatalln(err)
		return
	}

	for i := 0; i < 10; i++ {
		body := fmt.Sprintf("Hello World! Message #%d", i+1)

		err := ch.Publish(
			"",     // exchange
			q.Name, // routing key (큐 이름)
			false,  // mandatory
			false,  // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
			})
		if err != nil {
			log.Fatalf("%s: %s\n", "Failed to publish a message", err)
		}

		fmt.Printf(" [x] Sent %s\n", body)
		time.Sleep(1 * time.Second)
	}
}
