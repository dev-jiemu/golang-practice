package main

import (
	socketio "github.com/googollee/go-socket.io"
	"golangChat/src/chat"
)

func main() {

	server := socketio.NewServer(nil)

	go chat.Chatroom()

	server.OnConnect("/", func(s socketio.Conn) error {
		// 웹 브라우저에 접속되면
		var c chat.Subscription
		c = chat.Subscribe()
		chat.Join(s.ID())

		for _, event := range c.Archive {
			s.Emit("event", event)
		}

		//newMessage := make(chan string)

		return nil
	})

}
