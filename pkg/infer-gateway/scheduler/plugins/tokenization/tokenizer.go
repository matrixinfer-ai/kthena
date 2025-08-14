package tokenization

import (
	"context"
	"fmt"
	"time"
)

const (
	defaultRemoteTimeout    = 30 * time.Second
	defaultRemoteMaxRetries = 3
)

type remoteTokenizerImpl struct {
	config  RemoteTokenizerConfig
	client  *httpClient
	adapter engineAdapter
}

func NewRemoteTokenizer(config RemoteTokenizerConfig) (Tokenizer, error) {
	if err := validateRemoteConfig(&config); err != nil {
		return nil, err
	}

	if config.Engine != "vllm" {
		return nil, fmt.Errorf("unsupported engine: %s, only vllm is supported", config.Engine)
	}

	adapter := newVLLMAdapter(config.Model)

	client := newHTTPClient(config.Endpoint, config.Timeout, config.MaxRetries)

	return &remoteTokenizerImpl{
		config:  config,
		client:  client,
		adapter: adapter,
	}, nil
}

func (t *remoteTokenizerImpl) TokenizeInputText(text string) ([]byte, error) {
	ctx := context.Background()
	input := TokenizeInput{
		Type:             CompletionInput,
		Text:             text,
		AddSpecialTokens: t.config.AddSpecialTokens,
	}

	result, err := t.TokenizeWithOptions(ctx, input)
	if err != nil {
		return nil, err
	}

	return intToByteArray(result.Tokens), nil
}

func (t *remoteTokenizerImpl) TokenizeWithOptions(ctx context.Context, input TokenizeInput) (*TokenizeResult, error) {
	request, err := t.adapter.PrepareTokenizeRequest(input)
	if err != nil {
		return nil, ErrTokenizationFailed{
			Message: "failed to prepare request",
			Cause:   err,
		}
	}

	path := t.adapter.GetTokenizePath()
	respData, err := t.client.Post(ctx, path, request)
	if err != nil {
		return nil, ErrTokenizationFailed{
			Message: "request failed",
			Cause:   err,
		}
	}

	result, err := t.adapter.ParseTokenizeResponse(respData)
	if err != nil {
		return nil, ErrTokenizationFailed{
			Message: "failed to parse response",
			Cause:   err,
		}
	}

	return result, nil
}

func (t *remoteTokenizerImpl) GetEndpoint() string {
	return t.config.Endpoint
}

func (t *remoteTokenizerImpl) IsHealthy(ctx context.Context) bool {
	testInput := TokenizeInput{
		Type:             CompletionInput,
		Text:             "",
		AddSpecialTokens: false,
	}

	_, err := t.TokenizeWithOptions(ctx, testInput)
	return err == nil
}

func (t *remoteTokenizerImpl) Close() error {
	if t.client != nil {
		t.client.Close()
	}
	return nil
}

func validateRemoteConfig(c *RemoteTokenizerConfig) error {
	if c.Engine == "" {
		return ErrInvalidConfig{Message: "Engine cannot be empty"}
	}
	if c.Endpoint == "" {
		return ErrInvalidConfig{Message: "Endpoint cannot be empty"}
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultRemoteTimeout
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = defaultRemoteMaxRetries
	}
	return nil
}

var _ remoteTokenizer = (*remoteTokenizerImpl)(nil)
