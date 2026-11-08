// Package packetqueue bounds a userspace device's egress backlog and rotates
// active packet flows without reordering packets within a recognized flow.
package packetqueue

import (
	"container/list"
	"os"
	"sync"

	"github.com/xtls/xray-core/common/buf"
)

// Queue takes ownership only after Push succeeds. Pop transfers ownership to
// its caller; Close releases every packet still owned by the queue.
type Queue struct {
	mu       sync.Mutex
	readable *sync.Cond
	writable *sync.Cond
	active   list.List
	flows    map[flowKey]*list.Element
	capacity int
	count    int
	closed   bool
}

func New(capacity int) *Queue {
	if capacity < 1 {
		panic("packet queue capacity must be positive")
	}
	queue := &Queue{capacity: capacity, flows: make(map[flowKey]*list.Element)}
	queue.readable = sync.NewCond(&queue.mu)
	queue.writable = sync.NewCond(&queue.mu)
	return queue
}

func (q *Queue) Push(packet *buf.Buffer) error {
	key := classify(packet.Bytes())
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.count == q.capacity && !q.closed {
		q.writable.Wait()
	}
	if q.closed {
		return os.ErrClosed
	}
	element := q.flows[key]
	if element == nil {
		element = q.active.PushBack(&packetFlow{key: key})
		q.flows[key] = element
	}
	element.Value.(*packetFlow).push(packet)
	q.count++
	q.readable.Signal()
	return nil
}

func (q *Queue) Pop() (*buf.Buffer, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.count == 0 && !q.closed {
		q.readable.Wait()
	}
	if q.closed {
		return nil, os.ErrClosed
	}
	element := q.active.Front()
	flow := element.Value.(*packetFlow)
	packet := flow.pop()
	q.count--
	if flow.head == nil {
		q.active.Remove(element)
		delete(q.flows, flow.key)
	} else {
		q.active.MoveToBack(element)
	}
	q.writable.Signal()
	return packet, nil
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.count
}

func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	for _, element := range q.flows {
		flow := element.Value.(*packetFlow)
		for flow.head != nil {
			flow.pop().Release()
		}
	}
	clear(q.flows)
	q.active.Init()
	q.count = 0
	q.readable.Broadcast()
	q.writable.Broadcast()
}
