package queue

// Define a generic fixed-size structure with type parameter T
type FixedQueue[T any] struct {
	data  []T
	limit int
}

// Create a new FixedQueue with a specified limit (e.g., 5)
func NewFixedQueue[T any](limit int) *FixedQueue[T] {
	return &FixedQueue[T]{
		data:  make([]T, 0, limit),
		limit: limit,
	}
}

// Enqueue adds a new element to the queue.
// If the queue exceeds the limit, it removes the oldest element.
func (q *FixedQueue[T]) Enqueue(val T) T {
	var removedElement T
	if len(q.data) >= q.limit {
		removedElement = q.data[0]
		q.data = q.data[1:] // Remove the oldest element (first one)
	}

	q.data = append(q.data, val) // Add the new element
	return removedElement
}

// Values returns the current values in the queue.
func (q *FixedQueue[T]) Values() []T {
	return q.data
}
