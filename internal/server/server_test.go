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

func setupTestServer(t *testing.T, defaultConnectors []string, onTaskCreate func(req manus.CreateTaskRequest)) (*Server, *httptest.Server) {
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
			if onTaskCreate != nil {
				var parsedReq manus.CreateTaskRequest
				if err := json.NewDecoder(r.Body).Decode(&parsedReq); err == nil {
					onTaskCreate(parsedReq)
				}
			}
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
	srv := NewServer(p, client, nil, "max", defaultConnectors)

	return srv, ts
}

func TestServerGetPoolStatus(t *testing.T) {
	srv, ts := setupTestServer(t, nil, nil)
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
	srv, ts := setupTestServer(t, nil, nil)
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

func TestServerStopTask(t *testing.T) {
	srv, ts := setupTestServer(t, nil, nil)
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
	srv, ts := setupTestServer(t, nil, nil)
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

func TestServerCreateTaskWithConnectors(t *testing.T) {
	var capturedReq manus.CreateTaskRequest
	srv, ts := setupTestServer(t, nil, func(req manus.CreateTaskRequest) {
		capturedReq = req
	})
	defer ts.Close()

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "manus_create_task"
	req.Params.Arguments = map[string]any{
		"prompt":     "Use custom tools",
		"connectors": "mcp-router-novanodes, test-connector ", // Notice the trailing space for trim test
	}

	res, err := srv.handleCreateTask(ctx, req)
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	if len(capturedReq.Connectors) != 2 {
		t.Fatalf("expected 2 connectors, got %d: %v", len(capturedReq.Connectors), capturedReq.Connectors)
	}
	if capturedReq.Connectors[0] != "mcp-router-novanodes" || capturedReq.Connectors[1] != "test-connector" {
		t.Errorf("connectors mismatch: %v", capturedReq.Connectors)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "**Attached Connectors:** `mcp-router-novanodes`, `test-connector`") {
		t.Errorf("expected Attached Connectors in output, got:\n%s", text)
	}
}

func TestServerCreateTaskWithDefaultConnectorsFallback(t *testing.T) {
	var capturedReq manus.CreateTaskRequest
	defaultConnectors := []string{"default-conn-1", "default-conn-2"}
	srv, ts := setupTestServer(t, defaultConnectors, func(req manus.CreateTaskRequest) {
		capturedReq = req
	})
	defer ts.Close()

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "manus_create_task"
	req.Params.Arguments = map[string]any{
		"prompt": "Use default tools",
	}

	res, err := srv.handleCreateTask(ctx, req)
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	if len(capturedReq.Connectors) != 2 {
		t.Fatalf("expected 2 connectors from fallback, got %d: %v", len(capturedReq.Connectors), capturedReq.Connectors)
	}
	if capturedReq.Connectors[0] != "default-conn-1" || capturedReq.Connectors[1] != "default-conn-2" {
		t.Errorf("fallback connectors mismatch: %v", capturedReq.Connectors)
	}

	text := res.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "**Attached Connectors:** `default-conn-1`, `default-conn-2`") {
		t.Errorf("expected Attached Connectors from fallback in output, got:\n%s", text)
	}
}
