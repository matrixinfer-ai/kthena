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
