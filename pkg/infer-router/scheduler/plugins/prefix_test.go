/*
Copyright The Volcano Authors.

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

package plugins

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cespare/xxhash"
)

func TestHashPrompt(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		prompt         string
		blockSize      int
		maxBlocks      int
		expectedHashes []uint64
	}{
		{
			name:           "Empty prompt",
			model:          "test-model",
			prompt:         "",
			blockSize:      64,
			maxBlocks:      128,
			expectedHashes: []uint64{},
		},
		{
			name:      "Single block prompt",
			model:     "test-model",
			prompt:    "Hello World",
			blockSize: 64,
			maxBlocks: 128,
			expectedHashes: []uint64{
				xxhash.Sum64([]byte(fmt.Sprintf("%dHello World", xxhash.Sum64([]byte("test-model"))))),
			},
		},
		{
			name:      "Multi block prompt",
			model:     "test-model",
			prompt:    "This is a longer prompt that should span multiple blocks based on the block size",
			blockSize: 20,
			maxBlocks: 128,
			expectedHashes: []uint64{
				xxhash.Sum64([]byte(fmt.Sprintf("%dThis is a longer pro", xxhash.Sum64([]byte("test-model"))))),
				xxhash.Sum64([]byte(fmt.Sprintf("%dmpt that should span", xxhash.Sum64([]byte(fmt.Sprintf("%dThis is a longer pro", xxhash.Sum64([]byte("test-model")))))))),
				xxhash.Sum64([]byte(fmt.Sprintf("%d multiple blocks bas", xxhash.Sum64([]byte(fmt.Sprintf("%dmpt that should span", xxhash.Sum64([]byte(fmt.Sprintf("%dThis is a longer pro", xxhash.Sum64([]byte("test-model"))))))))))),
				xxhash.Sum64([]byte(fmt.Sprintf("%ded on the block size", xxhash.Sum64([]byte(fmt.Sprintf("%d multiple blocks bas", xxhash.Sum64([]byte(fmt.Sprintf("%dmpt that should span", xxhash.Sum64([]byte(fmt.Sprintf("%dThis is a longer pro", xxhash.Sum64([]byte("test-model")))))))))))))),
			},
		},
		{
			name:      "Max blocks limit",
			model:     "test-model",
			prompt:    "A very long prompt " + strings.Repeat("test ", 100),
			blockSize: 10,
			maxBlocks: 3,
			expectedHashes: []uint64{
				xxhash.Sum64([]byte(fmt.Sprintf("%dA very lon", xxhash.Sum64([]byte("test-model"))))),
				xxhash.Sum64([]byte(fmt.Sprintf("%dg prompt t", xxhash.Sum64([]byte(fmt.Sprintf("%dA very lon", xxhash.Sum64([]byte("test-model")))))))),
				xxhash.Sum64([]byte(fmt.Sprintf("%dest test t", xxhash.Sum64([]byte(fmt.Sprintf("%dg prompt t", xxhash.Sum64([]byte(fmt.Sprintf("%dA very lon", xxhash.Sum64([]byte("test-model"))))))))))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PrefixCache{
				blockSizeToHash:  tt.blockSize,
				maxBlocksToMatch: tt.maxBlocks,
			}
			got := p.hashPrompt(tt.model, tt.prompt)

			if !reflect.DeepEqual(got, tt.expectedHashes) {
				t.Errorf("hashPrompt() = %v, want %v", got, tt.expectedHashes)
			}
		})
	}
}
