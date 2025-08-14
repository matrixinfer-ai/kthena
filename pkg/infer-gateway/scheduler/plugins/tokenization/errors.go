package tokenization

import "fmt"

type ErrInvalidConfig struct {
	Message string
}

func (e ErrInvalidConfig) Error() string {
	return fmt.Sprintf("invalid config: %s", e.Message)
}

type ErrTokenizationFailed struct {
	Message string
	Cause   error
}

func (e ErrTokenizationFailed) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("tokenization failed: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("tokenization failed: %s", e.Message)
}

type ErrHTTPRequest struct {
	StatusCode int
	Message    string
	Cause      error
}

func (e ErrHTTPRequest) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("HTTP %d: %s: %v", e.StatusCode, e.Message, e.Cause)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}
