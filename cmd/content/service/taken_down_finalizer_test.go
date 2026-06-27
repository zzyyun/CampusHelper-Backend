package service

import (
	"testing"
	"time"
)

func TestTakenDownFinalizer_New(t *testing.T) {
	f := NewTakenDownFinalizer("amqp://test:test@localhost:5672/")
	if f == nil {
		t.Fatal("NewTakenDownFinalizer returned nil")
	}
	if f.mqAddr != "amqp://test:test@localhost:5672/" {
		t.Errorf("mqAddr mismatch: %s", f.mqAddr)
	}
}

func TestTakenDownFinalizer_Stop(t *testing.T) {
	f := NewTakenDownFinalizer("amqp://test:test@localhost:5672/")
	f.Stop() // 应该不 panic
}

func TestHasAppeal_DefaultFalse(t *testing.T) {
	// 当前实现默认返回 false（无申诉）
	if hasAppeal(123) {
		t.Error("default implementation should return false")
	}
}

func TestScanTakenDownPendingPosts_Stub(t *testing.T) {
	// 当前为 stub，返回 nil
	posts, err := scanTakenDownPendingPosts(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Errorf("scan should not error: %v", err)
	}
	if posts != nil {
		t.Errorf("stub should return nil, got %v", posts)
	}
}

func TestJoinStrings_Empty(t *testing.T) {
	result := joinStrings(nil, ",")
	if result != "" {
		t.Errorf("empty input should return empty string, got %s", result)
	}
}

func TestJoinStrings_Single(t *testing.T) {
	result := joinStrings([]string{"a"}, ",")
	if result != "a" {
		t.Errorf("single item should return 'a', got %s", result)
	}
}

func TestJoinStrings_Multiple(t *testing.T) {
	result := joinStrings([]string{"a", "b", "c"}, ",")
	if result != "a,b,c" {
		t.Errorf("multiple items should return 'a,b,c', got %s", result)
	}
}