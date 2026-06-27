package model

import (
	"testing"
)

func TestAIResult_String(t *testing.T) {
	tests := []struct {
		input    AIResult
		expected string
	}{
		{AIResultPass, "pass"},
		{AIResultReview, "review"},
		{AIResultBlock, "block"},
		{AIResult(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.input.String()
		if got != tt.expected {
			t.Errorf("AIResult(%d): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestAIStatus_String(t *testing.T) {
	tests := []struct {
		input    AIStatus
		expected string
	}{
		{AIStatusSynced, "synced"},
		{AIStatusDegraded, "degraded"},
		{AIStatusAsync, "async"},
		{AIStatus(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.input.String()
		if got != tt.expected {
			t.Errorf("AIStatus(%d): expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestAIAuditLog_TableName(t *testing.T) {
	log := AIAuditLog{}
	if log.TableName() != "ai_audit_logs" {
		t.Errorf("expected table name 'ai_audit_logs', got %q", log.TableName())
	}
}

func TestAIAuditLog_Fields(t *testing.T) {
	log := AIAuditLog{
		ID:           1234567890,
		PostID:       100,
		ContentHash:  "abc123",
		AIStatus:     AIStatusSynced,
		AIResult:     AIResultPass,
		AIConfidence: 0.95,
		AICategories: `[""]`,
		LatencyMs:    50,
		ModelVersion: "v1.0",
		FallbackUsed: false,
		TraceID:      "trace-abc",
	}
	if log.ID != 1234567890 {
		t.Error("ID not set")
	}
	if log.PostID != 100 {
		t.Error("PostID not set")
	}
	if log.AIStatus != AIStatusSynced {
		t.Error("AIStatus not set")
	}
	if log.AIResult != AIResultPass {
		t.Error("AIResult not set")
	}
	if log.AIConfidence != 0.95 {
		t.Error("AIConfidence not set")
	}
}