package manus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL default base URL for Manus API v2.
	DefaultBaseURL = "https://api.manus.ai"

	// DefaultTimeout default HTTP timeout for API calls.
	DefaultTimeout = 15 * time.Second
)

// Client executes API requests against api.manus.ai.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient initializes a new Manus API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetAvailableCredits calls GET /v2/usage.availableCredits.
func (c *Client) GetAvailableCredits(ctx context.Context, apiKey string) (*AvailableCredits, error) {
	endpoint := c.baseURL + "/v2/usage.availableCredits"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-manus-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	var resp AvailableCreditsResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, resp.Error
		}
		return nil, fmt.Errorf("unexpected error in availableCredits response")
	}
	return &resp.Data, nil
}

// CreateTask calls POST /v2/task.create.
func (c *Client) CreateTask(ctx context.Context, apiKey string, taskReq CreateTaskRequest) (*CreateTaskData, error) {
	endpoint := c.baseURL + "/v2/task.create"
	bodyBytes, err := json.Marshal(taskReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-manus-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var resp CreateTaskResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, resp.Error
		}
		return nil, fmt.Errorf("failed to create task")
	}
	return &resp.Data, nil
}

// ListMessages calls GET /v2/task.listMessages.
func (c *Client) ListMessages(ctx context.Context, apiKey, taskID string, order string, limit int) ([]TaskMessage, error) {
	if order == "" {
		order = "asc"
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	endpoint := fmt.Sprintf("%s/v2/task.listMessages?task_id=%s&order=%s&limit=%d",
		c.baseURL,
		url.QueryEscape(taskID),
		url.QueryEscape(order),
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-manus-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	var resp ListMessagesResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, resp.Error
		}
		return nil, fmt.Errorf("failed to list messages")
	}
	return resp.Data.Messages, nil
}

// GetTaskDetail calls GET /v2/task.detail.
func (c *Client) GetTaskDetail(ctx context.Context, apiKey, taskID string) (*TaskDetailData, error) {
	endpoint := fmt.Sprintf("%s/v2/task.detail?task_id=%s", c.baseURL, url.QueryEscape(taskID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-manus-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	var resp TaskDetailResponse
	if err := c.doRequest(req, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error != nil {
			return nil, resp.Error
		}
		return nil, fmt.Errorf("failed to get task detail")
	}
	return &resp.Data, nil
}

// StopTask calls POST /v2/task.stop.
func (c *Client) StopTask(ctx context.Context, apiKey, taskID string) error {
	endpoint := c.baseURL + "/v2/task.stop"
	payload := StopTaskRequest{TaskID: taskID}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal stop request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-manus-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var resp StandardResponse
	if err := c.doRequest(req, &resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error != nil {
			return resp.Error
		}
		return fmt.Errorf("failed to stop task")
	}
	return nil
}

// SendMessage calls POST /v2/task.sendMessage.
func (c *Client) SendMessage(ctx context.Context, apiKey string, reqData SendMessageRequest) error {
	endpoint := c.baseURL + "/v2/task.sendMessage"
	bodyBytes, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("marshal sendMessage request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-manus-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var resp StandardResponse
	if err := c.doRequest(req, &resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error != nil {
			return resp.Error
		}
		return fmt.Errorf("failed to send message")
	}
	return nil
}

func (c *Client) doRequest(req *http.Request, target interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr StandardResponse
		_ = json.Unmarshal(bodyBytes, &apiErr)

		httpErr := &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
			APIError:   apiErr.Error,
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return ErrRateLimited
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return ErrUnauthorized
		}
		if resp.StatusCode == http.StatusNotFound {
			return ErrTaskNotFound
		}
		return httpErr
	}

	if target != nil {
		if err := json.Unmarshal(bodyBytes, target); err != nil {
			return fmt.Errorf("unmarshal response (status %d): %w", resp.StatusCode, err)
		}
	}

	return nil
}
