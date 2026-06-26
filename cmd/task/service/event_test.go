package service

import (
	"testing"

	"go_projects/praProject1/pkg/mq"
)

func TestTaskEventConstants(t *testing.T) {
	if mq.EventTaskCreated != "task.created" {
		t.Errorf("EventTaskCreated = %q", mq.EventTaskCreated)
	}
	if mq.EventTaskClaimed != "task.claimed" {
		t.Errorf("EventTaskClaimed = %q", mq.EventTaskClaimed)
	}
	if mq.EventTaskCompleted != "task.completed" {
		t.Errorf("EventTaskCompleted = %q", mq.EventTaskCompleted)
	}
	if mq.EventTaskCancelled != "task.cancelled" {
		t.Errorf("EventTaskCancelled = %q", mq.EventTaskCancelled)
	}
	if mq.EventTaskExpired != "task.expired" {
		t.Errorf("EventTaskExpired = %q", mq.EventTaskExpired)
	}
}

func TestIsTaskNotificationEvent(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{mq.EventTaskClaimed, true},
		{mq.EventTaskCompleted, true},
		{mq.EventTaskCancelled, true},
		{mq.EventTaskExpired, true},
		{mq.EventTaskCreated, false},
		{"unknown", false},
	}
	for _, tc := range tests {
		got := isTaskNotificationEvent(tc.eventType)
		if got != tc.want {
			t.Errorf("isTaskNotificationEvent(%q) = %v, want %v", tc.eventType, got, tc.want)
		}
	}
}
