package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Ref. https://medium.com/@mhrlife/long-polling-with-golang-158f73474cbc

type CappedQueue[T any] struct {
	items    []T
	lock     *sync.RWMutex
	capacity int
}

func NewCappedQueue[T any](capacity int) *CappedQueue[T] {
	return &CappedQueue[T]{
		items:    make([]T, 0, capacity),
		lock:     new(sync.RWMutex),
		capacity: capacity,
	}
}

func (q *CappedQueue[T]) Append(item T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if l := len(q.items); l == 0 { // 비어있으면 추가
		q.items = append(q.items, item)
	} else {
		to := q.capacity - 1
		if l < q.capacity {
			to = l // (**) 이부분임
		}
		q.items = append([]T{item}, q.items[:to]...) // item을 먼저 넣고 그 뒤에꺼 복사 = 나중에 용량 차서 지울때 앞에꺼로 지움 (**)
	}
}

// Copy queue 에 있는 message 를 copy 해서 return
func (q *CappedQueue[T]) Copy() []T {
	q.lock.RLock()
	defer q.lock.RUnlock()

	copied := make([]T, len(q.items))
	for i, item := range q.items {
		copied[i] = item
	}
	return copied
}

type SendMessageRequest struct {
	Message string `json:"message"`
}

type Update struct {
	CreatedAt int64
	Message   string
}

type PubSub struct {
	channels []chan struct{}
	lock     *sync.RWMutex
}

func NewPubSub() *PubSub {
	return &PubSub{
		channels: make([]chan struct{}, 0),
		lock:     new(sync.RWMutex),
	}
}

// Subscribe 새 채널 추가
func (p *PubSub) Subscribe() (<-chan struct{}, func()) {
	p.lock.Lock()
	defer p.lock.Unlock()

	c := make(chan struct{}, 1) // 새 채널 생성
	p.channels = append(p.channels, c)
	return c, func() {
		p.lock.Lock()
		defer p.lock.Unlock()

		for i, channel := range p.channels {
			if channel == c { // 생성한 채널을 닫음
				p.channels = append(p.channels[:i], p.channels[i+1:]...)
				close(c)
				return
			}
		}
	}
}

// Publish 이벤트 발생 알림
func (p *PubSub) Publish() {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, channel := range p.channels {
		channel <- struct{}{}
	}
}

func main() {
	q := NewCappedQueue[Update](10)
	ps := NewPubSub()

	e := echo.New()

	// example : http://localhost:8000/updates?lastUpdate=1710139265
	e.GET("updates", func(c echo.Context) error {
		lastUpdate := c.QueryParam("lastUpdate")
		lastUpdateUnix, _ := strconv.ParseInt(lastUpdate, 10, 64)
		getUpdates := func() []Update {
			return filter(q.Copy(), func(update Update) bool {
				return update.CreatedAt > lastUpdateUnix
			})
		}

		// show it to user if we already have an update
		if updates := getUpdates(); len(updates) > 0 {
			return c.JSON(200, updates)
		}
		ch, close := ps.Subscribe()
		defer close()

		select {
		case <-ch:
			return c.JSON(200, getUpdates())
		case <-c.Request().Context().Done():
			return c.String(http.StatusRequestTimeout, "timeout") // context 가 종료되었으면 timeout 응답
		}
	})

	// 특정 timestamp 기준으로 updates 에 요청한 후 send를 발생시키면 데이터가 표출됨
	e.POST("send", func(c echo.Context) error {
		var request SendMessageRequest
		if err := c.Bind(&request); err != nil {
			return c.String(400, fmt.Sprintf("Bad request: %v", err))
		}

		q.Append(Update{
			CreatedAt: time.Now().Unix(),
			Message:   request.Message,
		})

		ps.Publish()
		return c.JSON(201, "I've sent your request.")
	})

	e.Logger.Fatal(e.Start(":8000"))
}

// pkg.Filter가 도대체 뭔 패키지인지 모르겠어서 야매로 만듬
func filter(updates []Update, f func(Update) bool) []Update {
	var filteredUpdates []Update
	for _, update := range updates {
		if f(update) {
			filteredUpdates = append(filteredUpdates, update)
		}
	}
	return filteredUpdates
}
