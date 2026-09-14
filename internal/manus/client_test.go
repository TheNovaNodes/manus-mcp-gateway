package manus_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
)

func TestGetAvailableCredits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v2/usage.availableCredits" {
			t.Errorf("expected /v2/usage.availableCredits, got %s", r.URL.Path)
		}
		if r.Header.Get("x-manus-api-key") != "sk-test-key" {
			t.Errorf("unexpected api key: %s", r.Header.Get("x-manus-api-key"))
		}

		resp := manus.AvailableCreditsResponse{
			OK:        true,
			RequestID: "req_123",
			Data: &manus.AvailableCredits{
				TotalCredits:      300,
				RefreshCredits:    300,
				MaxRefreshCredits: 300,
				NextRefreshTime:   1726300000,
				RefreshInterval:   "daily",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	credits, err := client.GetAvailableCredits(ctx, "sk-test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if credits.TotalCredits != 300 {
		t.Errorf("expected 300 total credits, got %d", credits.TotalCredits)
	}
	if credits.RefreshInterval != "daily" {
		t.Errorf("expected daily refresh interval, got %s", credits.RefreshInterval)
	}
	if credits.NextRefreshTimeFormatted() == "N/A" {
		t.Errorf("expected formatted date, got N/A")
	}
}

func TestCreateTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/task.create" {
			t.Errorf("expected /v2/task.create, got %s", r.URL.Path)
		}

		var payload manus.CreateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}

		if payload.Message.Content != "do something" {
			t.Errorf("expected content 'do something', got %s", payload.Message.Content)
		}

		resp := manus.CreateTaskResponse{
			OK:        true,
			RequestID: "req_create",
			Data: &manus.CreateTaskData{
				TaskID:    "task_abc123",
				TaskTitle: "My Task",
				Status:    "pending",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := client.CreateTask(ctx, "sk-test-key", manus.CreateTaskRequest{
		Message:      manus.TaskMessageInput{Content: "do something"},
		AgentProfile: "manus-1.6-lite",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.TaskID != "task_abc123" {
		t.Errorf("expected task_abc123, got %s", data.TaskID)
	}
}

func TestRateLimitError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		resp := manus.StandardResponse{
			OK: false,
			Error: &manus.APIError{
				Code:    "rate_limited",
				Message: "Rate limit exceeded",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.GetAvailableCredits(ctx, "sk-test-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, manus.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}

	var rateLimitErr *manus.RateLimitError
	if errors.As(err, &rateLimitErr) {
		if rateLimitErr.RetryAfter != 120*time.Second {
			t.Errorf("expected RetryAfter 120s, got %v", rateLimitErr.RetryAfter)
		}
	} else {
		t.Errorf("expected RateLimitError, got %T", err)
	}
}

func TestRateLimitError_HTTPDate(t *testing.T) {
	futureTime := time.Now().Add(5 * time.Minute).UTC()
	dateStr := futureTime.Format(time.RFC1123)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", dateStr)
		w.WriteHeader(http.StatusTooManyRequests)
		resp := manus.StandardResponse{
			OK: false,
			Error: &manus.APIError{
				Code:    "rate_limited",
				Message: "Rate limit exceeded",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.GetAvailableCredits(ctx, "sk-test-key")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rateLimitErr *manus.RateLimitError
	if errors.As(err, &rateLimitErr) {
		if rateLimitErr.RetryAfter <= 0 {
			t.Errorf("expected positive RetryAfter, got %v", rateLimitErr.RetryAfter)
		}
	} else {
		t.Errorf("expected RateLimitError, got %T", err)
	}
}

func TestHTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errCode    string
		expectErr  error
	}{
		{"Unauthorized 401", http.StatusUnauthorized, "unauthorized", manus.ErrUnauthorized},
		{"Forbidden 403", http.StatusForbidden, "forbidden", manus.ErrUnauthorized},
		{"TaskNotFound 404", http.StatusNotFound, "task_not_found", manus.ErrTaskNotFound},
		{"ServerError 500", http.StatusInternalServerError, "server_error", manus.ErrServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				resp := manus.StandardResponse{
					OK: false,
					Error: &manus.APIError{
						Code:    tt.errCode,
						Message: tt.name,
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer ts.Close()

			client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := client.GetAvailableCredits(ctx, "sk-test-key")
			if !errors.Is(err, tt.expectErr) {
				t.Errorf("expected error %v, got %v", tt.expectErr, err)
			}
			if err != nil && err.Error() == "" {
				t.Errorf("expected non-empty error string")
			}
		})
	}

	rlZero := manus.RateLimitError{}
	if rlZero.Error() != "rate limit exceeded (429)" {
		t.Errorf("unexpected RateLimitError string: %s", rlZero.Error())
	}
}

func TestListMessages(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v2/task.listMessages" {
			t.Errorf("expected /v2/task.listMessages, got %s", r.URL.Path)
		}

		resp := manus.ListMessagesResponse{
			OK: true,
			Data: &manus.ListMessagesData{
				Messages: []manus.TaskMessage{
					{
						Type: "assistant_message",
						AssistantMessage: &manus.AssistantMessage{
							Text: "Hello!",
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgs, err := client.ListMessages(ctx, "sk-test-key", "task_1", "asc", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 1 || msgs[0].Type != "assistant_message" {
		t.Errorf("expected 1 assistant_message, got %d", len(msgs))
	}
}

func TestGetTaskDetail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v2/task.detail" {
			t.Errorf("expected /v2/task.detail, got %s", r.URL.Path)
		}

		resp := manus.TaskDetailResponse{
			OK: true,
			Data: &manus.TaskDetailData{
				ID:     "task_1",
				Status: "running",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	data, err := client.GetTaskDetail(ctx, "sk-test-key", "task_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.ID != "task_1" || data.Status != "running" {
		t.Errorf("expected task_1 / running, got %s / %s", data.ID, data.Status)
	}
}

func TestSendMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/task.sendMessage" {
			t.Errorf("expected /v2/task.sendMessage, got %s", r.URL.Path)
		}

		resp := manus.StandardResponse{OK: true}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.SendMessage(ctx, "sk-test-key", manus.SendMessageRequest{
		TaskID: "task_1",
		Message: manus.TaskMessageInput{
			Content: "hello",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/task.stop" {
			t.Errorf("expected /v2/task.stop, got %s", r.URL.Path)
		}

		resp := manus.StandardResponse{OK: true}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.StopTask(ctx, "sk-test-key", "task_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
