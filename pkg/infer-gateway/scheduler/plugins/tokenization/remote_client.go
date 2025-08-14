package tokenization

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
)

type httpClient struct {
	client     *http.Client
	baseURL    string
	maxRetries int
}

func newHTTPClient(baseURL string, timeout time.Duration, maxRetries int) *httpClient {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if maxRetries < 0 {
		maxRetries = defaultMaxRetries
	}

	return &httpClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL:    baseURL,
		maxRetries: maxRetries,
	}
}

func (c *httpClient) Post(ctx context.Context, path string, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}

		lastErr = ErrHTTPRequest{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}

		if !c.shouldRetry(resp.StatusCode) {
			break
		}

		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(seconds) * time.Second):
				}
			}
		}
	}

	return nil, lastErr
}

func (c *httpClient) shouldRetry(statusCode int) bool {
	switch statusCode {
	case 408, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

func (c *httpClient) Close() {
	if c.client != nil {
		c.client.CloseIdleConnections()
	}
}
