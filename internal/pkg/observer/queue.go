package observer

import "sync"

type queue[T any] struct {
	mutex sync.Mutex
	items []T
}

func (q *queue[T]) Enqueue(item T) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.items = append(q.items, item)
}

func (q *queue[T]) Dequeue() T {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if len(q.items) == 0 {
		var zero T
		return zero
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

func (q *queue[T]) Len() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.items)
}

func newQueue[T any]() *queue[T] {
	return &queue[T]{}
}

func (q *queue[T]) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.items = []T{}
}

func (q *queue[T]) All() []T {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	items := q.items
	q.items = []T{}
	return items
}
