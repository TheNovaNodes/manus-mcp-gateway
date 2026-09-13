package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
	"github.com/TheNovaNodes/manus-mcp-gateway/internal/pool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Server coordinates the Manus MCP gateway tools.
type Server struct {
	mcpServer *mcpserver.MCPServer
	pool      *pool.Pool
	client    *manus.Client
	logger    *slog.Logger
}

// NewServer creates and registers all Manus MCP tools.
func NewServer(p *pool.Pool, client *manus.Client, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	mcpSrv := mcpserver.NewMCPServer(
		"manus-mcp-gateway",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	s := &Server{
		mcpServer: mcpSrv,
		pool:      p,
		client:    client,
		logger:    logger,
	}

	s.registerTools()
	return s
}

// MCPServer returns the underlying mcpserver.MCPServer instance.
func (s *Server) MCPServer() *mcpserver.MCPServer {
	return s.mcpServer
}

func (s *Server) registerTools() {
	// 1. manus_get_pool_status
	s.mcpServer.AddTool(
		mcp.NewTool("manus_get_pool_status",
			mcp.WithDescription("Retrieve capacity pool status across all 7 Manus API accounts: remaining spendable credits, next refresh timers, and total available pool balance."),
		),
		s.handleGetPoolStatus,
	)

	// 2. manus_create_task
	s.mcpServer.AddTool(
		mcp.NewTool("manus_create_task",
			mcp.WithDescription("Delegate a task to Manus AI autonomous cloud agent. Uses Greedy Credit Routing to automatically pick the account with maximum credits, with automatic failover."),
			mcp.WithString("prompt", mcp.Required(), mcp.Description("Detailed task instructions for the Manus agent")),
			mcp.WithString("title", mcp.Description("Optional descriptive title for the task")),
			mcp.WithString("agent_profile", mcp.Description("Agent profile (default: 'manus-1.6-lite' for economical credit usage, or 'manus-1.6')")),
			mcp.WithString("key_id", mcp.Description("Force a specific account key ID (optional)")),
			mcp.WithString("project_id", mcp.Description("Optional Manus project ID")),
		),
		s.handleCreateTask,
	)

	// 3. manus_get_task_status
	s.mcpServer.AddTool(
		mcp.NewTool("manus_get_task_status",
			mcp.WithDescription("Check execution status, latest messages, and generated artifact links (files, websites, downloads) for a Manus task."),
			mcp.WithString("task_id", mcp.Required(), mcp.Description("The ID of the task to check")),
			mcp.WithString("key_id", mcp.Description("Account key ID if known. If omitted, searches across pool.")),
			mcp.WithString("order", mcp.Description("Message sorting order: 'asc' or 'desc' (default: 'asc')")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of messages to retrieve (default: 50)")),
		),
		s.handleGetTaskStatus,
	)

	// 4. manus_stop_task
	s.mcpServer.AddTool(
		mcp.NewTool("manus_stop_task",
			mcp.WithDescription("Kill-Switch: immediately stops a running Manus task to halt credit consumption."),
			mcp.WithString("task_id", mcp.Required(), mcp.Description("The ID of the task to stop")),
			mcp.WithString("key_id", mcp.Description("Account key ID if known")),
		),
		s.handleStopTask,
	)

	// 5. manus_send_message
	s.mcpServer.AddTool(
		mcp.NewTool("manus_send_message",
			mcp.WithDescription("Send a follow-up message to a Manus task for multi-turn conversations or answering agent questions."),
			mcp.WithString("task_id", mcp.Required(), mcp.Description("The ID of the task")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Follow-up instruction or response content")),
			mcp.WithString("key_id", mcp.Description("Account key ID if known")),
		),
		s.handleSendMessage,
	)
}

func (s *Server) handleGetPoolStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status, err := s.pool.GetPoolStatus(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetPoolStatus failed", "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get pool status: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("### 📊 Manus Capacity Pool Status\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Configured Keys:** %d\n", status.TotalKeys))
	sb.WriteString(fmt.Sprintf("- **Active & Healthy Keys:** %d\n", status.ActiveKeys))
	sb.WriteString(fmt.Sprintf("- **Total Spendable Credits:** **%d credits**\n", status.TotalCreditsPool))
	sb.WriteString(fmt.Sprintf("- **Last Verified:** %s\n\n", status.CheckedAt.UTC().Format(time.RFC3339)))

	sb.WriteString("| Account ID | Masked Key | Credits | Max Daily | Next Refresh (UTC) | Status |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")

	for _, k := range status.Keys {
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | **%d** | %d | %s | %s |\n",
			k.ID,
			k.MaskedKey,
			k.TotalCredits,
			k.MaxRefresh,
			k.NextRefreshFormatted(),
			k.Status,
		))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func (s *Server) handleCreateTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompt := strings.TrimSpace(req.GetString("prompt", ""))
	if prompt == "" {
		return mcp.NewToolResultError("Argument 'prompt' is required and cannot be empty"), nil
	}

	title := strings.TrimSpace(req.GetString("title", ""))
	if title == "" {
		title = "NovaNodes Autonomous Task"
	}

	agentProfile := strings.TrimSpace(req.GetString("agent_profile", ""))
	if agentProfile == "" {
		agentProfile = "manus-1.6-lite"
	}

	keyID := strings.TrimSpace(req.GetString("key_id", ""))
	projectID := strings.TrimSpace(req.GetString("project_id", ""))

	taskPayload := manus.CreateTaskRequest{
		Title:        title,
		Message:      manus.TaskMessageInput{Content: prompt},
		AgentProfile: agentProfile,
		ProjectID:    projectID,
	}

	// If explicit key requested, execute directly on it
	if keyID != "" {
		entry, err := s.pool.GetKeyByID(keyID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		data, err := s.client.CreateTask(ctx, entry.Key, taskPayload)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create task on key %s: %v", keyID, err)), nil
		}
		return s.formatTaskCreatedResult(data, entry), nil
	}

	// Greedy routing with automatic failover
	maxAttempts := s.pool.KeyCount()
	if maxAttempts <= 0 {
		return mcp.NewToolResultError("No API keys found in capacity pool"), nil
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		entry, err := s.pool.PickKey(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Key selection error: %v", err)), nil
		}

		data, err := s.client.CreateTask(ctx, entry.Key, taskPayload)
		if err == nil {
			s.logger.InfoContext(ctx, "Task created successfully", "task_id", data.TaskID, "key_id", entry.ID)
			return s.formatTaskCreatedResult(data, entry), nil
		}

		lastErr = err
		s.logger.WarnContext(ctx, "Key attempt failed, failing over", "key_id", entry.ID, "attempt", attempt, "error", err)

		if errors.Is(err, manus.ErrRateLimited) {
			s.pool.MarkRateLimited(entry.ID, 5*time.Minute)
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("All %d keys in capacity pool failed to create task. Last error: %v", maxAttempts, lastErr)), nil
}

func (s *Server) formatTaskCreatedResult(data *manus.CreateTaskData, entry *pool.KeyEntry) *mcp.CallToolResult {
	var sb strings.Builder
	sb.WriteString("✅ **Manus Task Dispatched Successfully!**\n\n")
	sb.WriteString(fmt.Sprintf("- **Task ID:** `%s`\n", data.TaskID))
	if data.TaskTitle != "" {
		sb.WriteString(fmt.Sprintf("- **Title:** %s\n", data.TaskTitle))
	}
	sb.WriteString(fmt.Sprintf("- **Assigned Key:** `%s` (`%s`)\n", entry.ID, entry.MaskedKey))
	sb.WriteString(fmt.Sprintf("- **Initial Status:** `%s`\n", data.Status))
	sb.WriteString("\n*Use `manus_get_task_status` with `task_id: \"" + data.TaskID + "\"` to poll for completion and inspect artifacts.*\n")

	return mcp.NewToolResultText(sb.String())
}

func (s *Server) handleGetTaskStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := strings.TrimSpace(req.GetString("task_id", ""))
	if taskID == "" {
		return mcp.NewToolResultError("Argument 'task_id' is required"), nil
	}

	keyID := strings.TrimSpace(req.GetString("key_id", ""))
	order := strings.TrimSpace(req.GetString("order", "asc"))
	limit := req.GetInt("limit", 50)

	var targetKey *pool.KeyEntry
	var err error

	if keyID != "" {
		targetKey, err = s.pool.GetKeyByID(keyID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	} else {
		targetKey, err = s.pool.FindKeyForTask(ctx, taskID)
		if err != nil {
			// Fallback: search by attempting ListMessages on keys until one succeeds
			targetKey, err = s.searchKeyByMessages(ctx, taskID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Could not find task %s on any key: %v", taskID, err)), nil
			}
		}
	}

	messages, err := s.client.ListMessages(ctx, targetKey.Key, taskID, order, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list messages for task %s: %v", taskID, err)), nil
	}

	// Aggregate status and attachments
	finalStatus := "running"
	var assistantTexts []string
	var attachments []manus.Attachment

	for _, msg := range messages {
		if msg.Type == "status_update" && msg.StatusUpdate != nil {
			if msg.StatusUpdate.AgentStatus == "stopped" {
				finalStatus = "completed (stopped)"
			} else if msg.StatusUpdate.AgentStatus != "" {
				finalStatus = msg.StatusUpdate.AgentStatus
			}
		} else if msg.Type == "assistant_message" && msg.AssistantMessage != nil {
			body := strings.TrimSpace(msg.AssistantMessage.Body())
			if body != "" {
				assistantTexts = append(assistantTexts, body)
			}
			if len(msg.AssistantMessage.Attachments) > 0 {
				attachments = append(attachments, msg.AssistantMessage.Attachments...)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 🤖 Manus Task Status: `%s`\n\n", taskID))
	sb.WriteString(fmt.Sprintf("- **Current Status:** `%s`\n", finalStatus))
	sb.WriteString(fmt.Sprintf("- **Account:** `%s` (`%s`)\n", targetKey.ID, targetKey.MaskedKey))
	sb.WriteString(fmt.Sprintf("- **Total Events Retrieved:** %d\n\n", len(messages)))

	if len(assistantTexts) > 0 {
		sb.WriteString("#### 💬 Latest Agent Output:\n")
		// Show the last assistant message
		lastText := assistantTexts[len(assistantTexts)-1]
		sb.WriteString(lastText + "\n\n")
	}

	if len(attachments) > 0 {
		sb.WriteString("#### 📎 Generated Artifacts & Attachments:\n")
		for idx, att := range attachments {
			title := att.Title
			if title == "" {
				title = fmt.Sprintf("Artifact #%d", idx+1)
			}
			sb.WriteString(fmt.Sprintf("- [%d] [%s](%s)\n", idx+1, title, att.URL))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func (s *Server) searchKeyByMessages(ctx context.Context, taskID string) (*pool.KeyEntry, error) {
	status, err := s.pool.GetPoolStatus(ctx)
	if err != nil {
		return nil, err
	}

	for _, k := range status.Keys {
		reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		msgs, err := s.client.ListMessages(reqCtx, k.Key, taskID, "asc", 1)
		cancel()

		if err == nil && len(msgs) > 0 {
			return k, nil
		}
	}
	return nil, fmt.Errorf("task %s not located in pool", taskID)
}

func (s *Server) handleStopTask(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := strings.TrimSpace(req.GetString("task_id", ""))
	if taskID == "" {
		return mcp.NewToolResultError("Argument 'task_id' is required"), nil
	}

	keyID := strings.TrimSpace(req.GetString("key_id", ""))
	var targetKey *pool.KeyEntry
	var err error

	if keyID != "" {
		targetKey, err = s.pool.GetKeyByID(keyID)
	} else {
		targetKey, err = s.searchKeyByMessages(ctx, taskID)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Could not resolve key for task %s: %v", taskID, err)), nil
	}

	if err := s.client.StopTask(ctx, targetKey.Key, taskID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to stop task %s: %v", taskID, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("🛑 **Task Stopped!**\n\nManus task `%s` on key `%s` has been stopped immediately. Credit consumption has been halted.", taskID, targetKey.ID)), nil
}

func (s *Server) handleSendMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := strings.TrimSpace(req.GetString("task_id", ""))
	content := strings.TrimSpace(req.GetString("content", ""))
	if taskID == "" || content == "" {
		return mcp.NewToolResultError("Arguments 'task_id' and 'content' are required"), nil
	}

	keyID := strings.TrimSpace(req.GetString("key_id", ""))
	var targetKey *pool.KeyEntry
	var err error

	if keyID != "" {
		targetKey, err = s.pool.GetKeyByID(keyID)
	} else {
		targetKey, err = s.searchKeyByMessages(ctx, taskID)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Could not resolve key for task %s: %v", taskID, err)), nil
	}

	err = s.client.SendMessage(ctx, targetKey.Key, manus.SendMessageRequest{
		TaskID:  taskID,
		Message: manus.TaskMessageInput{Content: content},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to send message to task %s: %v", taskID, err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("💬 **Message Sent!**\n\nFollow-up message successfully delivered to task `%s`.", taskID)), nil
}
