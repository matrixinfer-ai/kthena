package tokenizer

import (
	"github.com/pkoukk/tiktoken-go"
	tiktokenloader "github.com/pkoukk/tiktoken-go-loader"
)

const encodingName = "cl100k_base"

type TickToken struct{}

func (t *TickToken) CalculateTokenNum(prompt string) (int, error) {
	tiktoken.SetBpeLoader(tiktokenloader.NewOfflineLoader())
	encoding, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return 0, err
	}
	return len(encoding.Encode(prompt, nil, nil)), nil
}
