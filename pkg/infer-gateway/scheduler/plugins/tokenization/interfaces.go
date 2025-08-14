package tokenization

import "context"

type Tokenizer interface {
	TokenizeInputText(string) ([]byte, error)
}

type extendedTokenizer interface {
	Tokenizer
	TokenizeWithOptions(ctx context.Context, input TokenizeInput) (*TokenizeResult, error)
}

type remoteTokenizer interface {
	extendedTokenizer
	GetEndpoint() string
	IsHealthy(ctx context.Context) bool
	Close() error
}

type engineAdapter interface {
	PrepareTokenizeRequest(input TokenizeInput) (interface{}, error)
	ParseTokenizeResponse(data []byte) (*TokenizeResult, error)
	GetTokenizePath() string
}
