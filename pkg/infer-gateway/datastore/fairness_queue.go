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
	"context"
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
	UserID      string  // User ID for fairness scheduling
	ModelName   string  // Target model for per-model fair queuing
	Priority    float64 // Priority (lower value means higher priority)
	RequestTime time.Time
	NotifyChan  chan struct{}
}

// RequestPriorityQueue implements the heap.Interface
type RequestPriorityQueue struct {
	mu   sync.RWMutex // Ensure concurrent safety with read/write locks
	heap []*Request   // Underlying storage structure
	cond *sync.Cond   // Condition variable to signal availability
}

func NewRequestPriorityQueue() *RequestPriorityQueue {
	pq := &RequestPriorityQueue{
		heap: make([]*Request, 0),
	}
	pq.cond = sync.NewCond(&pq.mu)
	return pq
}

// Implement heap.Interface methods
func (pq *RequestPriorityQueue) Len() int { return len(pq.heap) }

func (pq *RequestPriorityQueue) Less(i, j int) bool {
	// same user, FIFO
	if pq.heap[i].UserID == pq.heap[j].UserID {
		return pq.heap[i].RequestTime.Before(pq.heap[j].RequestTime)
	}
	// different users, compare priority, actually token usage here
	if pq.heap[i].Priority != pq.heap[j].Priority {
		return pq.heap[i].Priority < pq.heap[j].Priority
	}
	// When priorities are equal, compare request arrival times: earlier times have higher priority
	return pq.heap[i].RequestTime.Before(pq.heap[j].RequestTime)
}

func (pq *RequestPriorityQueue) Swap(i, j int) {
	pq.heap[i], pq.heap[j] = pq.heap[j], pq.heap[i]
}

func (pq *RequestPriorityQueue) Push(x interface{}) {
	item := x.(*Request)
	pq.heap = append(pq.heap, item)
}

func (pq *RequestPriorityQueue) Pop() interface{} {
	n := len(pq.heap)
	if n == 0 {
		return nil
	}
	item := pq.heap[n-1]
	pq.heap[n-1] = nil
	pq.heap = pq.heap[0 : n-1]
	return item
}

func (pq *RequestPriorityQueue) PushRequest(r *Request) error {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	heap.Push(pq, r)
	// Signal that a new item is available
	if pq.cond != nil {
		pq.cond.Signal()
	}
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

// popWhenAvailable blocks until an item is available or the context is done, then pops one item.
func (pq *RequestPriorityQueue) popWhenAvailable(ctx context.Context) (*Request, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	for len(pq.heap) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		pq.cond.Wait() // releases lock and waits; re-acquires before returning
	}
	req := heap.Pop(pq).(*Request)
	return req, nil
}

func (pq *RequestPriorityQueue) Run(ctx context.Context, qps int) {
	if qps <= 0 {
		qps = 1 // prevent division by zero; or treat as unlimited with a fast ticker
	}
	interval := time.Second / time.Duration(qps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req, err := pq.popWhenAvailable(ctx)
			if err != nil {
				return
			}
			// Optional: notify producer that request is dequeued
			if req != nil && req.NotifyChan != nil {
				// Closing signals once; ensure only consumer closes it.
				close(req.NotifyChan)
			}
		}
	}
}
