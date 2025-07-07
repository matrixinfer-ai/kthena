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

package tokenizer

import (
	"math"
)

type SimpleEstimateTokenizer struct {
	CharactersPerToken float64
}

func NewSimpleEstimateTokenizer() Tokenizer {
	return &SimpleEstimateTokenizer{
		CharactersPerToken: 4.0,
	}
}

func (s *SimpleEstimateTokenizer) CalculateTokenNum(prompt string) (int, error) {
	// TODO: estimate token
	if prompt == "" {
		return 0, nil
	}
	return int(math.Ceil(float64(len(prompt)) / s.CharactersPerToken)), nil
}
