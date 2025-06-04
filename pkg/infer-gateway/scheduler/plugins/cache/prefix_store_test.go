package cache

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore"
	"matrixinfer.ai/matrixinfer/pkg/infer-gateway/datastore/testutil"
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
			mockStore := testutil.NewMockStore()
			store := NewModelPrefixStore(mockStore, tt.maxHashes, tt.topK)

			// Add pods to cache
			for i, pod := range tt.pods {
				if i < len(tt.addHashes) {
					store.Add(tt.model, tt.addHashes[i], pod)
				}
			}

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
				gotName := fmt.Sprintf("%s/%s", match.Pod.Pod.Namespace, match.Pod.Pod.Name)
				if gotName != expectedName {
					t.Errorf("match[%d]: got pod %s, want %s", i, gotName, expectedName)
				}
				if match.MatchLen != tt.expectedLens[i] {
					t.Errorf("match[%d]: got length %d, want %d", i, match.MatchLen, tt.expectedLens[i])
				}
			}
		})
	}
}
