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

package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
)

func TestModelPrefixStore(t *testing.T) {
	tests := []struct {
		name         string
		maxHashes    int
		topK         int
		model        string
		pods         []*datastore.PodInfo
		addHashes    [][]uint64 // hashes to add for each pod
		queryHashes  []uint64   // hashes to query
		expectedPods []string   // expected pod names in order
		expectedLens []int      // expected match lengths
	}{
		{
			name:      "Empty cache returns no matches",
			maxHashes: 100,
			topK:      3,
			model:     "test-model",
			pods: []*datastore.PodInfo{
				{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"}}},
			},
			queryHashes:  []uint64{1, 2, 3},
			expectedPods: []string{},
			expectedLens: []int{},
		},
		{
			name:      "Single pod exact match",
			maxHashes: 100,
			topK:      3,
			model:     "test-model",
			pods: []*datastore.PodInfo{
				{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"}}},
			},
			addHashes:    [][]uint64{{1, 2, 3}},
			queryHashes:  []uint64{1, 2, 3},
			expectedPods: []string{"ns1/pod1"},
			expectedLens: []int{3},
		},
		{
			name:      "Multiple pods with different match lengths",
			maxHashes: 100,
			topK:      3,
			model:     "test-model",
			pods: []*datastore.PodInfo{
				{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"}}},
				{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2", Namespace: "ns1"}}},
				{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod3", Namespace: "ns1"}}},
			},
			addHashes: [][]uint64{
				{1, 2, 3}, // pod1: full match
				{1, 2},    // pod2: partial match
				{1, 4, 5}, // pod3: single match
			},
			queryHashes:  []uint64{1, 2, 3},
			expectedPods: []string{"ns1/pod1", "ns1/pod2", "ns1/pod3"},
			expectedLens: []int{3, 2, 1},
		},
		{
			name:      "LRU eviction",
			maxHashes: 2, // Only allow 2 hashes
			topK:      3,
			model:     "test-model",
			pods: []*datastore.PodInfo{
				{Pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns1"}}},
			},
			addHashes:    [][]uint64{{1, 2, 3}}, // Add 3 hashes, one should be evicted
			queryHashes:  []uint64{1, 2, 3},
			expectedPods: []string{"ns1/pod1"},
			expectedLens: []int{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := datastore.New()
			store := NewModelPrefixStore(mockStore, tt.maxHashes, tt.topK)

			// Add pods to cache
			for i, pod := range tt.pods {
				if i < len(tt.addHashes) {
					store.Add(tt.model, tt.addHashes[i], pod)
				}
			}

			time.Sleep(50 * time.Millisecond) // Ensure cache evict cb has been run
			// Query matches
			matches := store.FindTopMatches(tt.model, tt.queryHashes, tt.pods)

			// Verify results
			if len(matches) != len(tt.expectedPods) {
				t.Errorf("got %d matches, want %d", len(matches), len(tt.expectedPods))
			}

			for i, match := range matches {
				if i >= len(tt.expectedPods) {
					break
				}
				expectedName := tt.expectedPods[i]
				if match.NamespacedName.String() != expectedName {
					t.Errorf("match[%d]: got pod %s, want %s", i, match.NamespacedName.String(), expectedName)
				}
				if match.MatchLen != tt.expectedLens[i] {
					t.Errorf("match[%d]: got length %d, want %d", i, match.MatchLen, tt.expectedLens[i])
				}
			}
		})
	}
}

func TestModelPrefixStoreConcurrency(t *testing.T) {
	t.Run("Concurrent Add operations", func(t *testing.T) {
		mockStore := datastore.New()
		store := NewModelPrefixStore(mockStore, 100, 10)

		const numGoroutines = 50
		const numHashesPerPod = 10

		// Create pods
		pods := make([]*datastore.PodInfo, numGoroutines)
		for i := range numGoroutines {
			pods[i] = &datastore.PodInfo{
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("pod%d", i),
						Namespace: "test",
					},
				},
			}
		}

		// Concurrently add hashes
		wg := sync.WaitGroup{}
		wg.Add(numGoroutines)
		for i := range numGoroutines {
			go func(podIndex int) {
				defer wg.Done()

				// Generate hashes for this pod
				hashes := make([]uint64, numHashesPerPod)
				for j := range numHashesPerPod {
					hashes[j] = uint64(podIndex*numHashesPerPod + j + 1)
				}

				store.Add("test-model", hashes, pods[podIndex])
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		// Verify data integrity - query for some hashes
		queryHashes := []uint64{1, 2, 3, 4, 5}
		matches := store.FindTopMatches("test-model", queryHashes, pods)
		assert.Equal(t, 1, len(matches))
	})

	t.Run("Concurrent Add and FindTopMatches", func(t *testing.T) {
		mockStore := datastore.New()
		store := NewModelPrefixStore(mockStore, 100, 5)

		const numWriters = 20
		const numReaders = 30

		// Create pods
		pods := make([]*datastore.PodInfo, numWriters)
		for i := range numWriters {
			pods[i] = &datastore.PodInfo{
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("writer-pod%d", i),
						Namespace: "test",
					},
				},
			}
		}

		var wg sync.WaitGroup
		wg.Add(numWriters + numReaders)
		// Start writers
		for i := range numWriters {
			go func(podIndex int) {
				defer wg.Done()
				hashes := []uint64{
					uint64(podIndex + 1),
					uint64(podIndex + 2),
					uint64(podIndex + 3),
				}
				store.Add("concurrent-model", hashes, pods[podIndex])
			}(i)
		}

		// Start readers
		for range numReaders {
			go func() {
				defer wg.Done()

				queryHashes := []uint64{1, 2, 3, 4, 5}
				matches := store.FindTopMatches("concurrent-model", queryHashes, pods)
				_ = matches // Just consume the result
			}()
		}

		// Let them run for a while
		wg.Wait()

		// Final verification
		queryHashes := []uint64{1, 2, 3}
		matches := store.FindTopMatches("concurrent-model", queryHashes, pods)
		// Should not panic and should return valid results
		for _, match := range matches {
			if match.MatchLen <= 0 {
				t.Errorf("Invalid match length: %d", match.MatchLen)
			}
		}
	})

	t.Run("Concurrent Add and Pod Deletion", func(t *testing.T) {
		mockStore := datastore.New()
		store := NewModelPrefixStore(mockStore, 50, 10)

		const numPods = 20
		const numOperations = 50

		// Create pods
		pods := make([]*datastore.PodInfo, numPods)
		for i := range numPods {
			pods[i] = &datastore.PodInfo{
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("deletion-pod%d", i),
						Namespace: "test",
					},
				},
			}
		}

		var wg sync.WaitGroup

		// Add operations
		for i := range numOperations {
			wg.Add(1)
			go func(opIndex int) {
				defer wg.Done()
				podIndex := opIndex % numPods
				hashes := []uint64{
					uint64(opIndex + 1),
					uint64(opIndex + 2),
					uint64(opIndex + 3),
				}
				store.Add("deletion-model", hashes, pods[podIndex])
			}(i)
		}

		time.Sleep(10 * time.Millisecond) // Let some adds happen first

		// Pod deletion operations
		for i := range numPods / 2 {
			wg.Add(1)
			go func(podIndex int) {
				defer wg.Done()

				nsName := types.NamespacedName{
					Namespace: pods[podIndex].Pod.Namespace,
					Name:      pods[podIndex].Pod.Name,
				}

				// Simulate pod deletion event
				store.onPodDeleted(datastore.EventData{
					EventType: datastore.EventDelete,
					Pod:       nsName,
				})
			}(i)
		}

		wg.Wait()

		queryHashes := []uint64{1, 2, 3}
		matches := store.FindTopMatches("deletion-model", queryHashes, pods)
		assert.Equal(t, 0, len(matches))
	})

	t.Run("LRU Eviction Under Concurrency", func(t *testing.T) {
		mockStore := datastore.New()
		// Small capacity to force frequent evictions
		store := NewModelPrefixStore(mockStore, 5, 10)

		const numPods = 10
		const numOperations = 100

		// Create pods
		pods := make([]*datastore.PodInfo, numPods)
		for i := range numPods {
			pods[i] = &datastore.PodInfo{
				Pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("eviction-pod%d", i),
						Namespace: "test",
					},
				},
			}
		}

		var wg sync.WaitGroup

		// Concurrent operations that will trigger evictions
		for i := range numOperations {
			wg.Add(1)
			go func(opIndex int) {
				defer wg.Done()
				podIndex := opIndex % numPods

				// Generate many hashes to trigger evictions
				hashes := make([]uint64, 10)
				for j := 0; j < 10; j++ {
					hashes[j] = uint64(opIndex*10 + j + 1)
				}

				store.Add("eviction-model", hashes, pods[podIndex])

				// Also do some queries
				if opIndex%3 == 0 {
					queryHashes := []uint64{
						uint64(opIndex + 1),
						uint64(opIndex + 2),
					}
					matches := store.FindTopMatches("eviction-model", queryHashes, pods)
					_ = matches
				}
			}(i)
		}

		wg.Wait()

		// Wait for all eviction callbacks to complete
		time.Sleep(100 * time.Millisecond)

		// Verify cache is still functional
		queryHashes := []uint64{990, 991, 992} // Recent hashes
		matches := store.FindTopMatches("eviction-model", queryHashes, pods)
		assert.Equal(t, 0, len(matches))
	})

	t.Run("High Load Stress Test", func(t *testing.T) {
		mockStore := datastore.New()
		store := NewModelPrefixStore(mockStore, 100, 20)

		const numModels = 5
		const numPodsPerModel = 10
		const numGoroutines = 100
		const duration = 200 * time.Millisecond

		// Create pods for each model
		allPods := make([][]*datastore.PodInfo, numModels)
		for m := range numModels {
			allPods[m] = make([]*datastore.PodInfo, numPodsPerModel)
			for p := range numPodsPerModel {
				allPods[m][p] = &datastore.PodInfo{
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("stress-model%d-pod%d", m, p),
							Namespace: "test",
						},
					},
				}
			}
		}

		stop := make(chan struct{})
		var wg sync.WaitGroup

		// Mixed workload: adds, queries, and deletions
		for i := range numGoroutines {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for {
					select {
					case <-stop:
						return
					default:
						modelIndex := goroutineID % numModels
						podIndex := goroutineID % numPodsPerModel
						modelName := fmt.Sprintf("stress-model-%d", modelIndex)

						operation := goroutineID % 4
						switch operation {
						case 0, 1: // Add operations (50% of the time)
							hashes := []uint64{
								uint64(goroutineID + 1),
								uint64(goroutineID + 2),
								uint64(goroutineID + 3),
							}
							store.Add(modelName, hashes, allPods[modelIndex][podIndex])

						case 2: // Query operations (25% of the time)
							queryHashes := []uint64{
								uint64((goroutineID % 50) + 1),
								uint64((goroutineID % 50) + 2),
							}
							matches := store.FindTopMatches(modelName, queryHashes, allPods[modelIndex])
							_ = matches

						case 3: // Pod deletion (25% of the time)
							if goroutineID%10 == 0 { // Only some goroutines do deletions
								nsName := types.NamespacedName{
									Namespace: allPods[modelIndex][podIndex].Pod.Namespace,
									Name:      allPods[modelIndex][podIndex].Pod.Name,
								}
								store.onPodDeleted(datastore.EventData{
									EventType: datastore.EventDelete,
									Pod:       nsName,
								})
							}
						}

						// Small delay to avoid overwhelming
						if goroutineID%10 == 0 {
							time.Sleep(time.Microsecond * 100)
						}
					}
				}
			}(i)
		}

		// Let the stress test run
		time.Sleep(duration)
		close(stop)
		wg.Wait()

		// Wait for cleanup operations
		time.Sleep(50 * time.Millisecond)

		// Final verification - cache should still be functional
		for m := range numModels {
			modelName := fmt.Sprintf("stress-model-%d", m)
			queryHashes := []uint64{4, 5, 6}
			matches := store.FindTopMatches(modelName, queryHashes, allPods[m])
			t.Logf("Model %s: found %d matches after stress test", modelName, len(matches))
		}
	})
}
