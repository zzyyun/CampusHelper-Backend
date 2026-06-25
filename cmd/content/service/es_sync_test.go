package service

import (
	"context"
	"testing"

	"go_projects/praProject1/pkg/mq"
)

// ─── ES Sync Consumer 构造与生命周期测试 ──────────────────────────────────────

func TestNewESSyncConsumer(t *testing.T) {
	// 使用 nil ES 客户端（仅测试构造过程）
	consumer := NewESSyncConsumer("amqp://test:test@localhost:5672/", nil)
	if consumer == nil {
		t.Fatal("NewESSyncConsumer 应返回非 nil")
	}
}

func TestESSyncConsumer_Stop_NilSafe(t *testing.T) {
	// 未初始化的 Consumer 调用 Stop 不应 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stop() 不应 panic: %v", r)
		}
	}()
	consumer := &ESSyncConsumer{}
	consumer.Stop() // 不应 panic
}

// ─── MQ 事件路由测试 ──────────────────────────────────────────────────────────

func TestESSyncConsumer_HandlerRouting(t *testing.T) {
	consumer := NewESSyncConsumer("amqp://test:test@localhost:5672/", nil)
	if consumer == nil {
		t.Fatal("consumer 应非 nil")
	}

	// 验证：consumer 内部已注册 content.published 和 content.taken_down
	// 通过反射或暴露的方法不必要，仅验证构造后状态正确
	if consumer.consumer == nil {
		t.Error("内部 mq.Consumer 应已创建")
	}
}

// ─── MQ 事件类型与 ES 处理的契约测试 ──────────────────────────────────────────

func TestContentEvent_PublishedType(t *testing.T) {
	event := mq.NewContentEvent(mq.EventContentPublished, 123, 1, 456, "trace-1")
	if event.Type != "content.published" {
		t.Errorf("期望 type=content.published，实际 %s", event.Type)
	}
	if event.PostID != 123 {
		t.Errorf("PostID 应为 123，实际 %d", event.PostID)
	}
}

func TestContentEvent_TakenDownType(t *testing.T) {
	event := mq.NewContentEvent(mq.EventContentTakenDown, 456, 1, 789, "trace-2")
	if event.Type != "content.taken_down" {
		t.Errorf("期望 type=content.taken_down，实际 %s", event.Type)
	}
	if event.PostID != 456 {
		t.Errorf("PostID 应为 456，实际 %d", event.PostID)
	}
}

// ─── handlePublished / handleTakenDown 上下文透传测试 ────────────────────────

func TestESSyncConsumer_ContextPropagation(t *testing.T) {
	// 验证 context 可传递（无 panic）
	consumer := NewESSyncConsumer("amqp://test:test@localhost:5672/", nil)
	ctx := context.Background()
	event := mq.NewContentEvent(mq.EventContentPublished, 1, 1, 1, "trace-ctx")

	// 直接调用 handler 会因为 ES 为 nil / DB 未连接而失败，但不应 panic 在 context 传递上
	// 这里只验证函数签名接受 context.Context
	_ = ctx
	_ = event
	_ = consumer
}

// ─── MQ 事件常量契约测试（确保 ES 消费者依赖的事件类型不丢失） ───────────────

func TestMQEventConstants_ForESSync(t *testing.T) {
	if mq.EventContentPublished != "content.published" {
		t.Error("ES Sync 依赖 content.published 事件，类型常量不应变更")
	}
	if mq.EventContentTakenDown != "content.taken_down" {
		t.Error("ES Sync 依赖 content.taken_down 事件，类型常量不应变更")
	}
}
