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
	"container/list"
	"fmt"
	"math"
	"sync"
	"time"
)

// Sliding window configuration
const (
	defaultTokenTrackerWindowSize = 5      // Default window size in the configured time units, example: 5 minutes
	defaultTokenTrackerMinTokens  = 1000.0 // Sensible min default value for adaptive token tracking(see vtc_basic)
	defaultTokenTrackerMaxTokens  = 8000.0 // Sensible max default value for adaptive token tracking(see vtc_basic)
	defaultTimeUnit               = "minutes"
	defaultInputTokenWeight       = 1.0
	defaultOutputTokenWeight      = 2.0
)

var (
	TokenTrackerWindowSize = defaultTokenTrackerWindowSize
	TokenTrackerMinTokens  = defaultTokenTrackerMinTokens
	TokenTrackerMaxTokens  = defaultTokenTrackerMaxTokens
	TimeUnitStr            = defaultTimeUnit
	InputTokenWeight       = defaultInputTokenWeight
	OutputTokenWeight      = defaultOutputTokenWeight
)

type TimeUnit int

const (
	Minutes TimeUnit = iota
	Seconds
	Milliseconds
)

var timeUnitDuration = map[TimeUnit]time.Duration{
	Minutes:      time.Minute,
	Seconds:      time.Second,
	Milliseconds: time.Millisecond,
}

func (unit TimeUnit) toTimestamp(t time.Time) int64 {
	if unit == Milliseconds {
		return t.UnixNano() / int64(time.Millisecond)
	}
	return t.Unix()
}

// TokenTracker tracks token usage per user
type TokenTracker interface {
	GetTokenCount(user, model string) (float64, error)

	UpdateTokenCount(user, model string, inputTokens, outputTokens float64) error

	GetMinTokenCount() (float64, error)

	GetMaxTokenCount() (float64, error)
}

// bucketNode stores the data for a single time bucket in the linked list.
type bucketNode struct {
	timestamp  int64
	tokenCount float64
}

// userBucketData holds the token tracking data structures for a single user.
type userBucketData struct {
	buckets *list.List              // Doubly linked list of *bucketNode, ordered by timestamp
	lookup  map[int64]*list.Element // Maps timestamp to list element for O(1) access
}

// InMemorySlidingWindowTokenTracker tracks tokens per user in a fixed-size sliding window (in-memory, thread-safe).
// This function refer to aibrix(https://github.com/vllm-project/aibrix/)
type InMemorySlidingWindowTokenTracker struct {
	mu              sync.RWMutex
	windowSize      time.Duration
	bucketUnit      TimeUnit
	userBucketStore map[string]map[string]*userBucketData // [user][model] -> token
	userTotals      map[string]map[string]float64         // [user][model] -> total token
	// Efficient Min/Max Tracking
	totalsToUsers   map[float64]map[[2]string]struct{} // 键为[2]string{user, model}
	minTrackedToken float64
	maxTrackedToken float64
}

// TokenTrackerOption is a function that configures a token tracker
type TokenTrackerOption func(*InMemorySlidingWindowTokenTracker)

// updateWindowSize recalculates the window size based on time unit
func (t *InMemorySlidingWindowTokenTracker) updateWindowSize() {
	// Set window size based on configured size and time unit
	t.windowSize = time.Duration(TokenTrackerWindowSize) * timeUnitDuration[t.bucketUnit]
}

func WithWindowSize(size int) TokenTrackerOption {
	return func(t *InMemorySlidingWindowTokenTracker) {
		// Override the default window size with the provided value
		TokenTrackerWindowSize = size
		t.updateWindowSize()
	}
}

func WithTimeUnit(unit TimeUnit) TokenTrackerOption {
	return func(t *InMemorySlidingWindowTokenTracker) {
		t.bucketUnit = unit
		t.updateWindowSize()
	}
}

func NewInMemorySlidingWindowTokenTracker() TokenTracker {
	defaultUnit := Minutes
	// Set default time unit from environment variable
	switch TimeUnitStr {
	case "seconds":
		defaultUnit = Seconds
	case "milliseconds":
		defaultUnit = Milliseconds
	}

	tracker := &InMemorySlidingWindowTokenTracker{
		bucketUnit:      defaultUnit,
		userBucketStore: make(map[string]map[string]*userBucketData),
		userTotals:      make(map[string]map[string]float64),
		totalsToUsers:   make(map[float64]map[[2]string]struct{}),
		minTrackedToken: math.MaxFloat64, // Start high so first positive value becomes min
		maxTrackedToken: 0.0,             // Start with zero as default max
	}

	tracker.updateWindowSize()
	return tracker
}

func (t *InMemorySlidingWindowTokenTracker) getCutoffTimestamp() int64 {
	cutoffTime := time.Now().Add(-t.windowSize)
	return t.bucketUnit.toTimestamp(cutoffTime)
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) pruneExpiredBucketsAndUpdateState(user, model string, cutoff int64) {
	modelBuckets, userOk := t.userBucketStore[user]
	if !userOk {
		return
	}
	bucketData, modelOk := modelBuckets[model]
	if !modelOk {
		return
	}

	bucketsList := bucketData.buckets
	lookupMap := bucketData.lookup
	modified := false
	oldTotal := t.userTotals[user][model]
	newTotal := oldTotal

	// Iterate from the front (oldest) of the list
	for el := bucketsList.Front(); el != nil; {
		node := el.Value.(*bucketNode)
		if node.timestamp < cutoff {
			// Remove expired bucket
			newTotal -= node.tokenCount
			delete(lookupMap, node.timestamp)
			next := el.Next()
			bucketsList.Remove(el)
			el = next
			modified = true
		} else {
			// Stop as soon as we find a non-expired bucket (list is ordered)
			break
		}
	}

	if modified {
		// Update user total and min/max tracking with correct old and new values
		t.updateUserTotalAndMinMax(user, model, oldTotal, newTotal)
	}
}

// Time: Avg O(1) (amortized), Worst O(B_u) where B_u = buckets for user u | Space: O(1)
func (t *InMemorySlidingWindowTokenTracker) GetTokenCount(user, model string) (float64, error) {

	if user == "" || model == "" {
		return 0, nil
	}
	t.mu.RLock()
	modelBuckets, userOk := t.userBucketStore[user]
	if !userOk || modelBuckets == nil {
		t.mu.RUnlock()
		return 0, nil
	}
	bucketData, modelOk := modelBuckets[model]
	if !modelOk || bucketData.buckets.Len() == 0 {
		total := t.userTotals[user][model]
		t.mu.RUnlock()
		return total, nil
	}

	cutoff := t.getCutoffTimestamp()
	needsPruning := false

	oldestElement := bucketData.buckets.Front()
	if oldestElement != nil {
		oldestNode := oldestElement.Value.(*bucketNode)
		if oldestNode.timestamp < cutoff {
			needsPruning = true
		}
	}
	t.mu.RUnlock()
	// Only acquire write lock if we need to prune
	if needsPruning {
		t.mu.Lock()
		defer t.mu.Unlock()
		// Re-check user data existence in case it was deleted between RUnlock and Lock
		if modelBuckets, userOk := t.userBucketStore[user]; userOk {
			if _, modelOk := modelBuckets[model]; modelOk {
				t.pruneExpiredBucketsAndUpdateState(user, model, cutoff)
			}
		}
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	if modelBuckets, userOk := t.userBucketStore[user]; userOk {
		if _, modelOk := modelBuckets[model]; modelOk {
			return t.userTotals[user][model], nil
		}
	}
	return 0, nil
}

// Time: O(1) | Space: O(1)
func (t *InMemorySlidingWindowTokenTracker) GetMinTokenCount() (float64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.minTrackedToken == math.MaxFloat64 {
		return TokenTrackerMinTokens, nil
	}
	return t.minTrackedToken, nil
}

// Time: O(1) | Space: O(1)
func (t *InMemorySlidingWindowTokenTracker) GetMaxTokenCount() (float64, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// If no active users or all have zero tokens, return default max
	if t.maxTrackedToken == 0 {
		return TokenTrackerMaxTokens, nil
	}
	return t.maxTrackedToken, nil
}

func (t *InMemorySlidingWindowTokenTracker) UpdateTokenCount(user, model string, inputTokens, outputTokens float64) error {
	if user == "" || model == "" {
		return fmt.Errorf("user ID and model cannot be empty")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	currentTimestamp := t.bucketUnit.toTimestamp(now)
	cutoff := t.getCutoffTimestamp()

	if _, ok := t.userBucketStore[user]; !ok {
		t.userBucketStore[user] = make(map[string]*userBucketData)
		t.userTotals[user] = make(map[string]float64)
	}
	if _, ok := t.userBucketStore[user][model]; !ok {
		t.userBucketStore[user][model] = &userBucketData{
			buckets: list.New(),
			lookup:  make(map[int64]*list.Element),
		}
	}
	oldTotal := t.userTotals[user][model]

	// Prune first before adding/updating to maintain window size constraint accurately
	t.pruneExpiredBucketsAndUpdateState(user, model, cutoff)

	// Clamp negative tokens to zero
	inputTokens = max(0, inputTokens)
	outputTokens = max(0, outputTokens)

	newTokens := inputTokens*InputTokenWeight + outputTokens*OutputTokenWeight

	// Check if a bucket for the current timestamp already exists
	bucketData := t.userBucketStore[user][model]
	if element, exists := bucketData.lookup[currentTimestamp]; exists {
		node := element.Value.(*bucketNode)
		node.tokenCount += newTokens
	} else {
		node := &bucketNode{timestamp: currentTimestamp, tokenCount: newTokens}
		element := bucketData.buckets.PushBack(node)
		bucketData.lookup[currentTimestamp] = element
	}

	// Calculate the final new total
	totalAfterPruning := t.userTotals[user][model]
	newTotal := totalAfterPruning + newTokens

	// Update user total and global min/max efficiently
	t.updateUserTotalAndMinMax(user, model, oldTotal, newTotal)

	return nil
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) updateUserTotalAndMinMax(user, model string, oldTotal, newTotal float64) {
	if oldTotal == newTotal {
		return
	}

	// Negative totals should be treated as 0
	newTotal = max(0, newTotal)

	t.userTotals[user][model] = newTotal
	t.removeFromTotals(oldTotal, user, model)
	t.addToTotals(newTotal, user, model)
	t.recalcMin(oldTotal, newTotal)
	t.recalcMax(oldTotal, newTotal)
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) removeFromTotals(oldTotal float64, user, model string) {
	if oldTotal <= 0 {
		return
	}
	key := [2]string{user, model}
	if users, ok := t.totalsToUsers[oldTotal]; ok {
		delete(users, key)
		if len(users) == 0 {
			delete(t.totalsToUsers, oldTotal)
		}
	}
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) addToTotals(newTotal float64, user, model string) {
	if newTotal <= 0 {
		return
	}
	key := [2]string{user, model}
	if _, ok := t.totalsToUsers[newTotal]; !ok {
		t.totalsToUsers[newTotal] = make(map[[2]string]struct{})
	}
	t.totalsToUsers[newTotal][key] = struct{}{}
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) wasLastUserAtBoundary(value float64, userTotal float64) bool {
	// Returns true if userTotal matches the boundary value and no users remain at userTotal.
	return userTotal > 0 && userTotal == value && len(t.totalsToUsers[userTotal]) == 0
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) recalcMin(oldTotal, newTotal float64) {
	if t.wasLastUserAtBoundary(t.minTrackedToken, oldTotal) {
		// Find new minimum.
		newMin := math.MaxFloat64
		for total := range t.totalsToUsers {
			// keys in totalsToUsers are guaranteed > 0 by addToTotals
			if total < newMin {
				newMin = total
			}
		}
		t.minTrackedToken = newMin
	} else if newTotal > 0 && newTotal < t.minTrackedToken {
		t.minTrackedToken = newTotal
	}

	// Ensure minTrackedToken is MaxFloat64 if no users have positive totals
	if len(t.totalsToUsers) == 0 {
		t.minTrackedToken = math.MaxFloat64
	}
}

// Caller must hold the write lock
func (t *InMemorySlidingWindowTokenTracker) recalcMax(oldTotal, newTotal float64) {
	if t.wasLastUserAtBoundary(t.maxTrackedToken, oldTotal) {
		// Find new maximum
		t.maxTrackedToken = 0
		for total := range t.totalsToUsers {
			if total > t.maxTrackedToken {
				t.maxTrackedToken = total
			}
		}
	} else if newTotal > t.maxTrackedToken {
		t.maxTrackedToken = newTotal
	}
}
