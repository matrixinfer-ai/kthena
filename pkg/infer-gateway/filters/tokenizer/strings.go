package tokenizer

import (
	"strings"
)

type StringsTokenizer struct{}

func (r *StringsTokenizer) CalculateTokenNum(prompt string) (int, error) {
	// Split the string, here use space to split, and do not implement BPE
	return len(strings.Fields(prompt)), nil
}
