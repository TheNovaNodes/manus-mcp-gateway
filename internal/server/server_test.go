package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
	"github.com/TheNovaNodes/manus-mcp-gateway/internal/pool"
	"github.com/mark3labs/mcp-go/mcp"
)

func setupTestServer(t *testing.T) (*Server, *httptest.Server) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v2/usage.availableCredits":
			resp := manus.AvailableCreditsResponse{
				OK: true,
				Data: &manus.AvailableCredits{
					TotalCredits:      300,
					RefreshCredits:    300,
					MaxRefreshCredits: 300,
					NextRefreshTime:   1726300000,
					RefreshInterval:   "daily",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/v2/task.create":
			resp := manus.CreateTaskResponse{
				OK: true,
				Data: &manus.CreateTaskData{
					TaskID:    "task_999",
					TaskTitle: "E2E Test Task",
					Status:    "pending",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/v2/task.listMessages":
			resp := manus.ListMessagesResponse{
				OK: true,
				Data: &manus.ListMessagesData{
					Messages: []manus.TaskMessage{
						{
							Type:         "status_update",
							StatusUpdate: &manus.StatusUpdateData{AgentStatus: "stopped"},
						},
						{
							Type: "assistant_message",
							AssistantMessage: &manus.AssistantMessage{
								Text: "Task complete. Generated report.",
								Attachments: []manus.Attachment{
									{URL: "https://manus.ai/download/report.pdf", Title: "Final Report"},
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/v2/task.detail":
			resp := manus.TaskDetailResponse{
				OK: true,
				Data: &manus.TaskDetailData{
					ID:     "task_999",
					Title:  "E2E Test Task",
					Status: "stopped",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/v2/task.stop":
			_ = json.NewEncoder(w).Encode(manus.StandardResponse{OK: true})

		case r.URL.Path == "/v2/task.sendMessage":
			_ = json.NewEncoder(w).Encode(manus.StandardResponse{OK: true})

		default:
			http.NotFound(w, r)
		}
	}))

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	keys := []*pool.KeyEntry{
		{ID: "TestAcc", Key: "sk-test-secret-key-1234"},
	}
	p := pool.NewPool(client, keys, 1*time.Minute)
	srv := NewServer(p, client, nil)
	
	_ = srv.MCPServer()

	return srv, ts
}

func TestServerGetPoolStatus(t *testing.T) {
	srv, ts := setupTestServer(t)
	defer ts.Close()

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "manus_get_pool_status"

	res, err := srv.handleGetPoolStatus(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Manus Capacity Pool Status") {
		t.Errorf("expected pool status header in output, got:\n%s", text)
	}
	if !strings.Contains(text, "300 credits") {
		t.Errorf("expected 300 credits in output, got:\n%s", text)
	}
}

func TestServerCreateAndGetTask(t *testing.T) {
	srv, ts := setupTestServer(t)
	defer ts.Close()

	ctx := context.Background()

	// 1. Create Task
	createReq := mcp.CallToolRequest{}
	createReq.Params.Name = "manus_create_task"
	createReq.Params.Arguments = map[string]any{
		"prompt": "Audit website and produce summary",
	}

	createRes, err := srv.handleCreateTask(ctx, createReq)
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	createText := createRes.Content[0].(mcp.TextContent).Text
	if !strings.Contains(createText, "task_999") {
		t.Errorf("expected task_999 in create output, got:\n%s", createText)
	}

	// 2. Get Task Status
	statusReq := mcp.CallToolRequest{}
	statusReq.Params.Name = "manus_get_task_status"
	statusReq.Params.Arguments = map[string]any{
		"task_id": "task_999",
	}

	statusRes, err := srv.handleGetTaskStatus(ctx, statusReq)
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	statusText := statusRes.Content[0].(mcp.TextContent).Text
	if !strings.Contains(statusText, "completed (stopped)") {
		t.Errorf("expected status 'completed (stopped)', got:\n%s", statusText)
	}
	if !strings.Contains(statusText, "https://manus.ai/download/report.pdf") {
		t.Errorf("expected artifact link in output, got:\n%s", statusText)
	}
}

func TestServerGetTaskStatus_OrderDesc(t *testing.T) {
	srv, ts := setupTestServer(t)
	defer ts.Close()

	ctx := context.Background()

	// Seed cache
	createReq := mcp.CallToolRequest{}
	createReq.Params.Name = "manus_create_task"
	createReq.Params.Arguments = map[string]any{"prompt": "Audit website and produce summary"}
	_, _ = srv.handleCreateTask(ctx, createReq)

	statusReqDesc := mcp.CallToolRequest{}
	statusReqDesc.Params.Name = "manus_get_task_status"
	statusReqDesc.Params.Arguments = map[string]any{
		"task_id": "task_999",
		"order":   "desc",
	}

	statusRes, err := srv.handleGetTaskStatus(ctx, statusReqDesc)
	if err != nil {
		t.Fatalf("status error desc: %v", err)
	}
	statusText := statusRes.Content[0].(mcp.TextContent).Text
	if !strings.Contains(statusText, "completed (stopped)") {
		t.Errorf("expected status 'completed (stopped)' for desc order, got:\n%s", statusText)
	}
}

func TestServerCreateTaskFailover(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/usage.availableCredits" {
			resp := manus.AvailableCreditsResponse{
				OK: true,
				Data: &manus.AvailableCredits{TotalCredits: 300},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/v2/task.create" {
			callCount++
			if callCount == 1 {
				// Simulate failure on first key
				w.WriteHeader(http.StatusTooManyRequests)
				resp := manus.StandardResponse{
					OK: false,
					Error: &manus.APIError{Code: "rate_limited", Message: "rate limit"},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Succeed on second key
			resp := manus.CreateTaskResponse{
				OK: true,
				Data: &manus.CreateTaskData{
					TaskID: "task_999",
					TaskTitle: "Failover Test Task",
					Status: "pending",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	keys := []*pool.KeyEntry{
		{ID: "FailAcc", Key: "sk-fail"},
		{ID: "SuccessAcc", Key: "sk-success"},
	}
	p := pool.NewPool(client, keys, 1*time.Minute)
	srv := NewServer(p, client, nil)

	ctx := context.Background()
	createReq := mcp.CallToolRequest{}
	createReq.Params.Name = "manus_create_task"
	createReq.Params.Arguments = map[string]any{"prompt": "Test failover"}
	
	res, err := srv.handleCreateTask(ctx, createReq)
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	
	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Task Dispatched Successfully") {
		t.Errorf("expected success after failover, got: %s", text)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestServerStopTask(t *testing.T) {
	srv, ts := setupTestServer(t)
	defer ts.Close()

	ctx := context.Background()
	stopReq := mcp.CallToolRequest{}
	stopReq.Params.Name = "manus_stop_task"
	stopReq.Params.Arguments = map[string]any{
		"task_id": "task_999",
	}

	stopRes, err := srv.handleStopTask(ctx, stopReq)
	if err != nil {
		t.Fatalf("stop error: %v", err)
	}
	stopText := stopRes.Content[0].(mcp.TextContent).Text
	if !strings.Contains(stopText, "Task Stopped!") {
		t.Errorf("expected Task Stopped confirmation, got:\n%s", stopText)
	}
}

func TestServerSendMessage(t *testing.T) {
	srv, ts := setupTestServer(t)
	defer ts.Close()

	ctx := context.Background()
	sendReq := mcp.CallToolRequest{}
	sendReq.Params.Name = "manus_send_message"
	sendReq.Params.Arguments = map[string]any{
		"task_id": "task_999",
		"content": "Please also export as CSV",
	}

	sendRes, err := srv.handleSendMessage(ctx, sendReq)
	if err != nil {
		t.Fatalf("send error: %v", err)
	}
	sendText := sendRes.Content[0].(mcp.TextContent).Text
	if !strings.Contains(sendText, "Message Sent!") {
		t.Errorf("expected Message Sent confirmation, got:\n%s", sendText)
	}
}

func TestSearchKeyByMessagesFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/usage.availableCredits":
			resp := manus.AvailableCreditsResponse{
				OK: true,
				Data: &manus.AvailableCredits{TotalCredits: 100},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v2/task.detail":
			// Detail fails
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(manus.StandardResponse{OK: false})
		case "/v2/task.listMessages":
			// Messages succeed
			resp := manus.ListMessagesResponse{
				OK: true,
				Data: &manus.ListMessagesData{
					Messages: []manus.TaskMessage{{Type: "assistant_message"}},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/v2/task.stop":
			_ = json.NewEncoder(w).Encode(manus.StandardResponse{OK: true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := manus.NewClient(manus.WithBaseURL(ts.URL), manus.WithHTTPClient(ts.Client()))
	keys := []*pool.KeyEntry{{ID: "FallbackAcc", Key: "sk-fallback"}}
	p := pool.NewPool(client, keys, 1*time.Minute)
	srv := NewServer(p, client, nil)

	ctx := context.Background()
	stopReq := mcp.CallToolRequest{}
	stopReq.Params.Name = "manus_stop_task"
	stopReq.Params.Arguments = map[string]any{"task_id": "task_fallback"}

	res, err := srv.handleStopTask(ctx, stopReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Task Stopped!") {
		t.Errorf("expected Task Stopped, got: %s", text)
	}
}
