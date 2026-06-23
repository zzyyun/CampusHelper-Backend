// Package contextx 提供跨服务/跨链路的 Context 扩展。
//
// 设计目标：
//   - 把 school_id / user_id / trace_id 等横切关注点显式注入 context
//   - 与 OpenTelemetry TraceID 共存：业务 trace_id 来自 HTTP Header "X-Trace-Id"
//   - 各中间件（Auth、Tracing、Logging）负责读写，业务代码通过 Get/Set 访问
package contextx

import "context"

// TraceIDKey context 中 trace_id 的 key 类型
type TraceIDKey struct{}

// UserIDKey context 中 user_id 的 key 类型
type UserIDKey struct{}

// SchoolIDKey context 中 school_id 的 key 类型
type SchoolIDKey struct{}

// WithTraceID 把 trace_id 写入 ctx，返回新 ctx
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, TraceIDKey{}, traceID)
}

// GetTraceID 从 ctx 提取 trace_id，未设置时返回 ""
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(TraceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// SetTraceID 把 trace_id 写入 ctx（与 WithTraceID 同义，提供动词别名便于阅读）
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return WithTraceID(ctx, traceID)
}

// WithUserID 把 user_id 写入 ctx
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, UserIDKey{}, userID)
}

// GetUserID 从 ctx 提取 user_id，未设置时返回 0
func GetUserID(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	if v, ok := ctx.Value(UserIDKey{}).(int64); ok {
		return v
	}
	return 0
}

// WithSchoolID 把 school_id 写入 ctx
func WithSchoolID(ctx context.Context, schoolID int64) context.Context {
	return context.WithValue(ctx, SchoolIDKey{}, schoolID)
}

// GetSchoolID 从 ctx 提取 school_id，未设置时返回 0
func GetSchoolID(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	if v, ok := ctx.Value(SchoolIDKey{}).(int64); ok {
		return v
	}
	return 0
}