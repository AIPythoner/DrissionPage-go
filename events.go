package drissionpage

import (
	"context"
	"sync"
)

// eventQueue keeps event delivery independent of consumers, so a slow consumer
// cannot block the CDP reader. Stop wakes all waiters; Clear releases snapshots.
type eventQueue[T any] struct {
	mu      sync.Mutex
	items   []T
	notify  chan struct{}
	closed  bool
	limit   int
	dropped uint64
}

func newEventQueue[T any]() *eventQueue[T] {
	return &eventQueue[T]{notify: make(chan struct{}), limit: 4096}
}

// QueueStats reports retained and discarded events. Overflow discards oldest events.
type QueueStats struct {
	Pending int
	Limit   int
	Dropped uint64
}

func (q *eventQueue[T]) stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	return QueueStats{len(q.items), q.limit, q.dropped}
}
func (q *eventQueue[T]) setLimit(limit int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if limit <= 0 {
		limit = 4096
	}
	q.limit = limit
	if len(q.items) > limit {
		n := len(q.items) - limit
		q.dropped += uint64(n)
		q.items = append([]T(nil), q.items[n:]...)
	}
}
func (q *eventQueue[T]) signal() { close(q.notify); q.notify = make(chan struct{}) }
func (q *eventQueue[T]) push(v T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		if q.limit > 0 && len(q.items) >= q.limit {
			var zero T
			q.items[0] = zero
			q.items = q.items[1:]
			q.dropped++
		}
		q.items = append(q.items, v)
		q.signal()
	}
}
func (q *eventQueue[T]) clear() { q.mu.Lock(); q.items = nil; q.mu.Unlock() }
func (q *eventQueue[T]) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		q.signal()
	}
}
func (q *eventQueue[T]) pop(ctx context.Context) (T, error) {
	var zero T
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			v := q.items[0]
			q.items[0] = zero
			q.items = q.items[1:]
			q.mu.Unlock()
			return v, nil
		}
		if q.closed {
			q.mu.Unlock()
			return zero, ErrClosed
		}
		ch := q.notify
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-ch:
		}
	}
}
