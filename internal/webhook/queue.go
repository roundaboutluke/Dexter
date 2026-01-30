package webhook

import "sync"

// Queue stores incoming webhook payloads.
type Queue struct {
	mu    sync.Mutex
	items []any
}

// NewQueue creates an empty queue.
func NewQueue() *Queue {
	return &Queue{items: make([]any, 0, 1024)}
}

// Push appends one or more items to the queue.
func (q *Queue) Push(items ...any) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, items...)
}

// Len returns the number of items in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Drain returns and clears queued items.
func (q *Queue) Drain() []any {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]any, len(q.items))
	copy(out, q.items)
	q.items = q.items[:0]
	return out
}
