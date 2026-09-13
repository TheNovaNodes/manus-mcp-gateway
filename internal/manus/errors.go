package manus

import (
	"errors"
	"fmt"
)

var (
	// ErrRateLimited indicates a 429 Too Many Requests response or rate limit code.
	ErrRateLimited = errors.New("rate limit exceeded (429)")

	// ErrUnauthorized indicates invalid or expired API key (401/403).
	ErrUnauthorized = errors.New("unauthorized / invalid API key")

	// ErrTaskNotFound indicates 404 task not found.
	ErrTaskNotFound = errors.New("task not found")

	// ErrNoCreditsAvailable indicates total_credits is 0.
	ErrNoCreditsAvailable = errors.New("no credits available on key")
)

// HTTPError encapsulates an unexpected HTTP status code.
type HTTPError struct {
	StatusCode int
	Body       string
	APIError   *APIError
}

func (e *HTTPError) Error() string {
	if e.APIError != nil {
		return fmt.Sprintf("HTTP %d: %s (%s)", e.StatusCode, e.APIError.Message, e.APIError.Code)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func (e *HTTPError) Is(target error) bool {
	if target == ErrRateLimited && e.StatusCode == 429 {
		return true
	}
	if target == ErrUnauthorized && (e.StatusCode == 401 || e.StatusCode == 403) {
		return true
	}
	if target == ErrTaskNotFound && e.StatusCode == 404 {
		return true
	}
	return false
}
