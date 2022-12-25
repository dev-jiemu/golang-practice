package chat

import (
	"container/list"
	"time"
)

// 채팅 이벤트 구조체
type Event struct {
	EvtType   string
	User      string
	Timestamp int
	Text      string
}

// 구독 구조체 정의
type Subscription struct {
	Archive []Event
	New     <-chan Event
}

func NewEvent(evtType, user, msg string) Event {
	return Event{
		EvtType:   evtType,
		User:      user,
		Timestamp: int(time.Now().Unix()),
		Text:      msg,
	}
}

var (
	subscribe   = make(chan (chan<- Subscription), 10) // 구독 채널
	unsubscribe = make(chan (<-chan Event), 10)        // 구독 해지 채널
	publish     = make(chan Event, 10)                 // 이벤트 발행 채널
)

// 새로운 사용자가 들어왔을 때 이벤트를 구독할 함수
func Subscribe() Subscription {
	c := make(chan Subscription)
	subscribe <- c
	return <-c
}

// 사용자가 나갔을때 구독을 취소할 함수
func (s Subscription) Cancel() {
	unsubscribe <- s.New

	for {
		select {
		case _, ok := <-s.New:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

// 사용자가 들어왔을때
func Join(user string) {
	publish <- NewEvent("join", user, "")
}

// 사용자가 채팅 메세지를 보냈을때
func Say(user, message string) {
	publish <- NewEvent("message", user, message)
}

// 사용자가 나갔을때
func Leave(user string) {
	publish <- NewEvent("leave", user, "")
}

// 구독, 구독해지, 발행된 이벤트를 처리
func Chatroom() {
	archive := list.New()     // 쌓인 이벤트 저장
	subscribers := list.New() // 구독자 목록 저장

	for {
		select {
		case c := <-subscribe: // 새 사용자가 들어왔다면
			var events []Event

			for e := archive.Front(); e != nil; e.Next() {
				events = append(events, e.Value.(Event))
			}

			subscriber := make(chan Event, 10)
			subscribers.PushBack(subscriber) // 이벤트 채널을 구독자 목록에 추가

			c <- Subscription{events, subscriber}

		case event := <-publish: //새 이벤트가 발생했을때
			for e := subscribers.Front(); e != nil; e.Next() {
				subscriber := e.Value.(chan Event) // 구독자 목록에서 이벤트 채널을 꺼냄

				subscriber <- event
			}

			// 저장된 이벤트가 20개가 넘어간다면 이벤트 삭제처리
			if archive.Len() >= 20 {
				archive.Remove(archive.Front())
			}
			archive.PushBack(event)

		case c := <-unsubscribe: // 사용자가 나갔을때
			for e := subscribers.Front(); e != nil; e.Next() {
				subscriber := e.Value.(chan Event)

				// 구독자 목록에 들어있는 이벤트와 채널이 같으면
				if subscriber == c {
					subscribers.Remove(e)
					break
				}
			}
		}
	}
}
