// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"errors"
	"github.com/globocom/tsuru/queue"
	"sync"
	"sync/atomic"
	"time"
)

var factory = NewFakeQFactory()

func init() {
	queue.Register("fake", factory)
}

type fakeHandler struct {
	running int32
}

func (h *fakeHandler) Start() {
	atomic.StoreInt32(&h.running, 1)
}

func (h *fakeHandler) Stop() error {
	if !atomic.CompareAndSwapInt32(&h.running, 1, 0) {
		return errors.New("Not running.")
	}
	return nil
}

func (h *fakeHandler) Wait() {}

type FakeQ struct {
	messages messageQueue
}

func (q *FakeQ) get(ch chan *queue.Message, stop chan int) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		if msg := q.messages.dequeue(); msg != nil {
			ch <- msg
			return
		}
		time.Sleep(1e3)
	}
}

func (q *FakeQ) Get(timeout time.Duration) (*queue.Message, error) {
	ch := make(chan *queue.Message, 1)
	stop := make(chan int, 1)
	defer close(stop)
	go q.get(ch, stop)
	select {
	case msg := <-ch:
		return msg, nil
	case <-time.After(timeout):
	}
	return nil, errors.New("Timed out.")
}

func (q *FakeQ) Put(m *queue.Message, delay time.Duration) error {
	if delay > 0 {
		go func() {
			time.Sleep(delay)
			q.messages.enqueue(m)
		}()
	} else {
		q.messages.enqueue(m)
	}
	return nil
}

func (q *FakeQ) Delete(m *queue.Message) error {
	return nil
}

func (q *FakeQ) Release(m *queue.Message, delay time.Duration) error {
	return q.Put(m, delay)
}

type FakeQFactory struct {
	queues map[string]*FakeQ
	sync.Mutex
}

func NewFakeQFactory() *FakeQFactory {
	return &FakeQFactory{
		queues: make(map[string]*FakeQ),
	}
}

func (f *FakeQFactory) Get(name string) (queue.Q, error) {
	f.Lock()
	defer f.Unlock()
	if q, ok := f.queues[name]; ok {
		return q, nil
	}
	q := FakeQ{}
	f.queues[name] = &q
	return &q, nil
}

func (f *FakeQFactory) Handler(fn func(*queue.Message), names ...string) (queue.Handler, error) {
	return &fakeHandler{}, nil
}

type messageNode struct {
	m    *queue.Message
	next *messageNode
	prev *messageNode
}

type messageQueue struct {
	first *messageNode
	last  *messageNode
	n     int
	sync.Mutex
}

func (q *messageQueue) enqueue(msg *queue.Message) {
	q.Lock()
	defer q.Unlock()
	if q.last == nil {
		q.last = &messageNode{m: msg}
		q.first = q.last
	} else {
		olast := q.last
		q.last = &messageNode{m: msg, prev: olast}
		olast.next = q.last
	}
	q.n++
}

func (q *messageQueue) dequeue() *queue.Message {
	q.Lock()
	defer q.Unlock()
	if q.n == 0 {
		return nil
	}
	msg := q.first.m
	q.n--
	q.first = q.first.next
	if q.n == 0 {
		q.last = q.first
	}
	return msg
}

// CleanQ deletes all messages from queues identified by the given names.
func CleanQ(names ...string) {
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			q, _ := factory.Get(name)
			for {
				msg, err := q.Get(1e6)
				if err != nil {
					break
				}
				q.Delete(msg)
			}
		}(name)
	}
	wg.Wait()
}
