package runner

import "sync"

// RingBuffer는 고정 용량을 가진 스레드 세이프 순환 버퍼입니다
type RingBuffer[T any] struct {
	data  []T
	size  int // 용량
	head  int // 다음 쓰기 위치
	count int // 현재 아이템 수
	mu    sync.RWMutex
}

// NewRingBuffer는 지정된 용량으로 새로운 링 버퍼를 생성합니다
func NewRingBuffer[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		data: make([]T, size),
		size: size,
		head: 0,
		count: 0,
	}
}

// Push는 버퍼에 아이템을 추가하며, 가득 찰 경우 가장 오래된 것을 덮어씁니다
func (rb *RingBuffer[T]) Push(item T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.data[rb.head] = item
	rb.head = (rb.head + 1) % rb.size

	if rb.count < rb.size {
		rb.count++
	}
}

// ToSlice는 모든 아이템을 순서대로 반환합니다 (가장 오래된 것부터 최신 것까지)
func (rb *RingBuffer[T]) ToSlice() []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return []T{}
	}

	result := make([]T, rb.count)

	if rb.count < rb.size {
		// 버퍼가 아직 가득 차지 않음, 아이템은 0부터 count-1까지
		copy(result, rb.data[:rb.count])
	} else {
		// 버퍼가 가득 챸, 가장 오래된 아이템은 head 위치에 있음
		// head부터 끝까지 복사
		n := copy(result, rb.data[rb.head:])
		// 시작부터 head까지 복사
		copy(result[n:], rb.data[:rb.head])
	}

	return result
}

// Len은 현재 아이템 수를 반환합니다
func (rb *RingBuffer[T]) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Clear는 모든 아이템을 제거합니다
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.head = 0
	rb.count = 0
	// 가비지 컨렉션을 허용하도록 데이터를 제로화
	for i := range rb.data {
		var zero T
		rb.data[i] = zero
	}
}
