/*
Copyright MatrixInfer-AI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package datastore

import (
	"container/heap"
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueEmpty = errors.New("queue is empty")
)

// Request represents a request item in the priority queue
type Request struct {
	ReqID       string
	UserID      string
	Payload     interface{} // Request payload data
	Priority    float64     // Priority (lower value means higher priority)
	Index       int         // Index in the heap
	RequestTime time.Time
	NotifyChan  chan struct{}
}

// RequestPriorityQueue implements the heap.Interface
type RequestPriorityQueue struct {
	mu   sync.RWMutex // Ensure concurrent safety with read/write locks
	heap []*Request   // Underlying storage structure
}

func NewRequestPriorityQueue() *RequestPriorityQueue {
	return &RequestPriorityQueue{
		heap: make([]*Request, 0),
	}
}

// Implement heap.Interface methods
func (pq *RequestPriorityQueue) Len() int {
	return len(pq.heap)
}

func (pq *RequestPriorityQueue) Less(i, j int) bool {
	if pq.heap[i].Priority != pq.heap[j].Priority {
		return pq.heap[i].Priority < pq.heap[j].Priority
	}
	// When priorities are equal, compare request arrival times: earlier times have higher priority
	return pq.heap[i].RequestTime.Before(pq.heap[j].RequestTime)
}

func (pq *RequestPriorityQueue) Swap(i, j int) {
	pq.heap[i], pq.heap[j] = pq.heap[j], pq.heap[i]
	pq.heap[i].Index = i
	pq.heap[j].Index = j
}

func (pq *RequestPriorityQueue) Push(x interface{}) {
	item := x.(*Request)
	item.Index = len(pq.heap)
	pq.heap = append(pq.heap, item)
}

func (pq *RequestPriorityQueue) Pop() interface{} {
	n := len(pq.heap)
	if n == 0 {
		return nil
	}
	item := pq.heap[n-1]
	pq.heap[n-1] = nil
	item.Index = -1
	pq.heap = pq.heap[0 : n-1]
	return item
}

func (pq *RequestPriorityQueue) PushRequest(r *Request) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	heap.Push(pq, r)
	return nil
}

func (pq *RequestPriorityQueue) PopRequest() (*Request, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if len(pq.heap) == 0 {
		return nil, ErrQueueEmpty
	}
	req := heap.Pop(pq).(*Request)
	return req, nil
}
