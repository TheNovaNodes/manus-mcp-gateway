package manus_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
)

func TestCustomInt64Unmarshal(t *testing.T) {
	var c manus.FlexTimestamp
	
	// Test string
	err := json.Unmarshal([]byte(`"1726300000"`), &c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if c.Int64() != 1726300000 {
		t.Errorf("expected 1726300000, got %d", c.Int64())
	}
	
	// Test int
	err = json.Unmarshal([]byte(`1726300000`), &c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if c.Int64() != 1726300000 {
		t.Errorf("expected 1726300000, got %d", c.Int64())
	}
}

func TestModelsConversionMethods(t *testing.T) {
	now := time.Now().Unix()
	
	credits := manus.AvailableCreditsResponse{
		Data: &manus.AvailableCredits{TotalCredits: 100},
	}
	if credits.ToCredits() == nil || credits.ToCredits().TotalCredits != 100 {
		t.Errorf("ToCredits failed")
	}
    emptyCredits := manus.AvailableCreditsResponse{}
    if emptyCredits.ToCredits() == nil || emptyCredits.ToCredits().TotalCredits != 0 {
        t.Errorf("ToCredits empty failed")
    }

	task := manus.CreateTaskResponse{
		Data: &manus.CreateTaskData{TaskID: "task_1"},
	}
	if task.ToTaskData() == nil || task.ToTaskData().TaskID != "task_1" {
		t.Errorf("ToTaskData failed")
	}
    emptyTask := manus.CreateTaskResponse{}
    if emptyTask.ToTaskData() == nil || emptyTask.ToTaskData().TaskID != "" {
        t.Errorf("ToTaskData empty failed")
    }

	msgs := manus.ListMessagesResponse{
		Data: &manus.ListMessagesData{
			Messages: []manus.TaskMessage{{Type: "assistant_message"}},
		},
	}
	if len(msgs.ToMessages()) != 1 {
		t.Errorf("ToMessages failed")
	}
    emptyMsgs := manus.ListMessagesResponse{}
    if msgs := emptyMsgs.ToMessages(); msgs != nil {
        t.Errorf("ToMessages empty failed")
    }
	
	detail := manus.TaskDetailResponse{
		Data: &manus.TaskDetailData{ID: "task_1"},
	}
	if detail.ToDetail() == nil || detail.ToDetail().ID != "task_1" {
		t.Errorf("ToDetail failed")
	}
    emptyDetail := manus.TaskDetailResponse{}
    if emptyDetail.ToDetail() == nil || emptyDetail.ToDetail().ID != "" {
        t.Errorf("ToDetail empty failed")
    }
	
	errResp := manus.StandardResponse{
		Error: &manus.APIError{Code: "test", Message: "test"},
	}
	if errResp.Error == nil || errResp.Error.Error() != "test: test" {
		t.Errorf("StandardResponse Error failed, got %v", errResp.Error)
	}

	msgWithText := manus.AssistantMessage{Text: "from text", Content: "from content"}
	if msgWithText.Body() != "from text" {
		t.Errorf("expected 'from text', got %s", msgWithText.Body())
	}
	msgWithContentOnly := manus.AssistantMessage{Content: "from content"}
	if msgWithContentOnly.Body() != "from content" {
		t.Errorf("expected 'from content', got %s", msgWithContentOnly.Body())
	}

	_ = now
}
