package model

import "testing"

func TestTaskStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   TaskStatus
		to     TaskStatus
		wantOk bool
	}{
		{"open → in_progress", TaskStatusOpen, TaskStatusInProgress, true},
		{"open → cancelled", TaskStatusOpen, TaskStatusCancelled, true},
		{"open → expired", TaskStatusOpen, TaskStatusExpired, true},
		{"in_progress → completed", TaskStatusInProgress, TaskStatusCompleted, true},
		{"in_progress → cancelled", TaskStatusInProgress, TaskStatusCancelled, true},
		{"open → completed (非法)", TaskStatusOpen, TaskStatusCompleted, false},
		{"completed → open (非法)", TaskStatusCompleted, TaskStatusOpen, false},
		{"expired → in_progress (非法)", TaskStatusExpired, TaskStatusInProgress, false},
		{"cancelled → open (非法)", TaskStatusCancelled, TaskStatusOpen, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.from.CanTransitionTo(tc.to)
			if got != tc.wantOk {
				t.Errorf("CanTransitionTo(%v → %v) = %v, 期望 %v", tc.from, tc.to, got, tc.wantOk)
			}
		})
	}
}

func TestTaskType_Values(t *testing.T) {
	if TaskTypeDelivery != "delivery" {
		t.Errorf("TaskTypeDelivery 应为 delivery，实际 %s", TaskTypeDelivery)
	}
	if TaskTypeCarpool != "carpool" {
		t.Errorf("TaskTypeCarpool 应为 carpool，实际 %s", TaskTypeCarpool)
	}
	if TaskTypeBounty != "bounty" {
		t.Errorf("TaskTypeBounty 应为 bounty，实际 %s", TaskTypeBounty)
	}
}

func TestTaskStatus_Values(t *testing.T) {
	if TaskStatusOpen != 1 {
		t.Errorf("TaskStatusOpen 应为 1，实际 %d", TaskStatusOpen)
	}
	if TaskStatusInProgress != 2 {
		t.Errorf("TaskStatusInProgress 应为 2，实际 %d", TaskStatusInProgress)
	}
	if TaskStatusCompleted != 3 {
		t.Errorf("TaskStatusCompleted 应为 3，实际 %d", TaskStatusCompleted)
	}
	if TaskStatusCancelled != 4 {
		t.Errorf("TaskStatusCancelled 应为 4，实际 %d", TaskStatusCancelled)
	}
	if TaskStatusExpired != 5 {
		t.Errorf("TaskStatusExpired 应为 5，实际 %d", TaskStatusExpired)
	}
}
