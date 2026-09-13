package manus

import (
	"encoding/json"
	"strconv"
	"time"
)

// FlexTimestamp parses Unix seconds/milliseconds from JSON string or integer.
type FlexTimestamp int64

// UnmarshalJSON implements custom JSON unmarshaling for string or int64 timestamps.
func (f *FlexTimestamp) UnmarshalJSON(data []byte) error {
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexTimestamp(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			*f = FlexTimestamp(parsed)
			return nil
		}
	}
	*f = 0
	return nil
}

// Int64 returns standard int64 representation.
func (f FlexTimestamp) Int64() int64 {
	return int64(f)
}

// AvailableCredits details from Manus API v2.
type AvailableCredits struct {
	TotalCredits      int           `json:"total_credits"`
	FreeCredits       int           `json:"free_credits"`
	PeriodicCredits   int           `json:"periodic_credits"`
	AddonCredits      int           `json:"addon_credits"`
	ProMonthlyCredits int           `json:"pro_monthly_credits"`
	EventCredits      int           `json:"event_credits"`
	RefreshCredits    int           `json:"refresh_credits"`
	MaxRefreshCredits int           `json:"max_refresh_credits"`
	NextRefreshTime   FlexTimestamp `json:"next_refresh_time"`
	RefreshInterval   string        `json:"refresh_interval"`
	CurrentPeriodEnd  FlexTimestamp `json:"current_period_end"`
}

// NextRefreshTimeFormatted returns formatted UTC date.
func (c *AvailableCredits) NextRefreshTimeFormatted() string {
	ts := c.NextRefreshTime.Int64()
	if ts <= 0 {
		return "N/A"
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

// AvailableCreditsResponse wraps both root fields and nested data envelope.
type AvailableCreditsResponse struct {
	OK                bool              `json:"ok"`
	RequestID         string            `json:"request_id,omitempty"`
	TotalCredits      int               `json:"total_credits"`
	FreeCredits       int               `json:"free_credits"`
	PeriodicCredits   int               `json:"periodic_credits"`
	AddonCredits      int               `json:"addon_credits"`
	ProMonthlyCredits int               `json:"pro_monthly_credits"`
	EventCredits      int               `json:"event_credits"`
	RefreshCredits    int               `json:"refresh_credits"`
	MaxRefreshCredits int               `json:"max_refresh_credits"`
	NextRefreshTime   FlexTimestamp     `json:"next_refresh_time"`
	RefreshInterval   string            `json:"refresh_interval"`
	CurrentPeriodEnd  FlexTimestamp     `json:"current_period_end"`
	Data              *AvailableCredits `json:"data,omitempty"`
	Error             *APIError         `json:"error,omitempty"`
}

// ToCredits extracts spendable credit structure from root or nested data.
func (r *AvailableCreditsResponse) ToCredits() *AvailableCredits {
	if r.Data != nil && r.Data.TotalCredits > 0 {
		return r.Data
	}
	return &AvailableCredits{
		TotalCredits:      r.TotalCredits,
		FreeCredits:       r.FreeCredits,
		PeriodicCredits:   r.PeriodicCredits,
		AddonCredits:      r.AddonCredits,
		ProMonthlyCredits: r.ProMonthlyCredits,
		EventCredits:      r.EventCredits,
		RefreshCredits:    r.RefreshCredits,
		MaxRefreshCredits: r.MaxRefreshCredits,
		NextRefreshTime:   r.NextRefreshTime,
		RefreshInterval:   r.RefreshInterval,
		CurrentPeriodEnd:  r.CurrentPeriodEnd,
	}
}

// CreateTaskRequest payload for POST /v2/task.create.
type CreateTaskRequest struct {
	Title            string                 `json:"title,omitempty"`
	Message          TaskMessageInput       `json:"message"`
	AgentProfile     string                 `json:"agent_profile,omitempty"`
	StructuredOutput map[string]interface{} `json:"structured_output,omitempty"`
	Connectors       []string               `json:"connectors,omitempty"`
	ProjectID        string                 `json:"project_id,omitempty"`
}

// TaskMessageInput message input for task.create or task.sendMessage.
type TaskMessageInput struct {
	Content     string   `json:"content"`
	Attachments []string `json:"attachments,omitempty"`
}

// CreateTaskResponse response from POST /v2/task.create (supports root and nested).
type CreateTaskResponse struct {
	OK        bool            `json:"ok"`
	RequestID string          `json:"request_id,omitempty"`
	TaskID    string          `json:"task_id"`
	TaskTitle string          `json:"task_title,omitempty"`
	TaskURL   string          `json:"task_url,omitempty"`
	Status    string          `json:"status,omitempty"`
	Data      *CreateTaskData `json:"data,omitempty"`
	Error     *APIError       `json:"error,omitempty"`
}

// ToTaskData extracts task metadata.
func (r *CreateTaskResponse) ToTaskData() *CreateTaskData {
	if r.Data != nil && r.Data.TaskID != "" {
		return r.Data
	}
	status := r.Status
	if status == "" {
		status = "pending"
	}
	return &CreateTaskData{
		TaskID:    r.TaskID,
		TaskTitle: r.TaskTitle,
		Status:    status,
	}
}

// CreateTaskData holds created task details.
type CreateTaskData struct {
	TaskID    string `json:"task_id"`
	TaskTitle string `json:"task_title,omitempty"`
	Status    string `json:"status,omitempty"`
}

// ListMessagesResponse response from GET /v2/task.listMessages (supports root and nested).
type ListMessagesResponse struct {
	OK        bool              `json:"ok"`
	RequestID string            `json:"request_id,omitempty"`
	TaskID    string            `json:"task_id,omitempty"`
	Messages  []TaskMessage     `json:"messages"`
	Data      *ListMessagesData `json:"data,omitempty"`
	Error     *APIError         `json:"error,omitempty"`
}

// ToMessages extracts message list.
func (r *ListMessagesResponse) ToMessages() []TaskMessage {
	if r.Data != nil && len(r.Data.Messages) > 0 {
		return r.Data.Messages
	}
	return r.Messages
}

// ListMessagesData messages wrapper.
type ListMessagesData struct {
	Messages []TaskMessage `json:"messages"`
}

// TaskMessage item inside task.listMessages.
type TaskMessage struct {
	ID               string            `json:"id,omitempty"`
	Type             string            `json:"type"` // status_update, assistant_message, user_message
	StatusUpdate     *StatusUpdateData `json:"status_update,omitempty"`
	AssistantMessage *AssistantMessage `json:"assistant_message,omitempty"`
	Timestamp        FlexTimestamp     `json:"timestamp,omitempty"`
	CreatedAt        string            `json:"created_at,omitempty"`
}

// StatusUpdateData status event details.
type StatusUpdateData struct {
	AgentStatus string `json:"agent_status"` // running, stopped, waiting
	Brief       string `json:"brief,omitempty"`
	Description string `json:"description,omitempty"`
}

// AssistantMessage content from Manus agent.
type AssistantMessage struct {
	Text        string       `json:"text,omitempty"`
	Content     string       `json:"content,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Body returns text or content if text is empty.
func (a *AssistantMessage) Body() string {
	if a.Text != "" {
		return a.Text
	}
	return a.Content
}

// Attachment file or URL artifact generated by the agent.
type Attachment struct {
	URL      string `json:"url"`
	Title    string `json:"title,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// TaskDetailResponse response from GET /v2/task.detail.
type TaskDetailResponse struct {
	OK        bool            `json:"ok"`
	RequestID string          `json:"request_id,omitempty"`
	ID        string          `json:"id"`
	Title     string          `json:"title,omitempty"`
	Status    string          `json:"status"`
	CreatedAt FlexTimestamp   `json:"created_at,omitempty"`
	UpdatedAt FlexTimestamp   `json:"updated_at,omitempty"`
	Data      *TaskDetailData `json:"data,omitempty"`
	Error     *APIError       `json:"error,omitempty"`
}

// ToDetail extracts detail data.
func (r *TaskDetailResponse) ToDetail() *TaskDetailData {
	if r.Data != nil && r.Data.ID != "" {
		return r.Data
	}
	return &TaskDetailData{
		ID:        r.ID,
		Title:     r.Title,
		Status:    r.Status,
		CreatedAt: r.CreatedAt.Int64(),
		UpdatedAt: r.UpdatedAt.Int64(),
	}
}

// TaskDetailData full metadata of a task.
type TaskDetailData struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

// StopTaskRequest payload for POST /v2/task.stop.
type StopTaskRequest struct {
	TaskID string `json:"task_id"`
}

// SendMessageRequest payload for POST /v2/task.sendMessage.
type SendMessageRequest struct {
	TaskID  string           `json:"task_id"`
	Message TaskMessageInput `json:"message"`
}

// StandardResponse generic response envelope.
type StandardResponse struct {
	OK        bool      `json:"ok"`
	RequestID string    `json:"request_id,omitempty"`
	Error     *APIError `json:"error,omitempty"`
}

// APIError standard error response format from Manus API.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Code + ": " + e.Message
}
