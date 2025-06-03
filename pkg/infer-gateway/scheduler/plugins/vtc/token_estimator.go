package vtc

import (
	"math"
)

type SimpleTokenEstimator struct {
	CharactersPerToken float64
	OutputRatio        float64
}


func NewSimpleTokenEstimator() TokenEstimator {
	return &SimpleTokenEstimator{
		CharactersPerToken: 4.0, // Rough approximation: 4 chars per token
		OutputRatio:        1.5, // Default output/input ratio
	}
}

func (e *SimpleTokenEstimator) EstimateInputTokens(message string) float64 {
	if message == "" {
		return 0
	}

	return math.Ceil(float64(len(message)) / e.CharactersPerToken)
}

func (e *SimpleTokenEstimator) EstimateOutputTokens(message string) float64 {
	inputTokens := e.EstimateInputTokens(message)
	return math.Ceil(inputTokens * e.OutputRatio)
}
