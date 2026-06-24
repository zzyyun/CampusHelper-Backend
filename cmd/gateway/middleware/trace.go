package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const traceIDHeader = "X-Trace-ID"

// Trace OTel Span 注入中间件。
//
// 增强点（Issue #23）：
//   - 客户端可在请求头携带 X-Trace-ID（32 字符 hex），网关优先使用该 ID 作为 TraceID
//   - 非法或缺失 X-Trace-ID 时回退到 OTel 默认行为（自动生成或从 traceparent 提取）
//   - 响应头 X-Trace-ID 始终返回当前 Span 的 TraceID，便于客户端关联日志
func Trace() gin.HandlerFunc {
	prop := otel.GetTextMapPropagator()
	tracer := otel.Tracer("gateway")

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 1. 检查自定义 X-Trace-ID header
		customID := c.GetHeader(traceIDHeader)

		if customID != "" {
			// 尝试解析为 32 字符 hex TraceID
			tid, err := trace.TraceIDFromHex(customID)
			if err != nil {
				log.Printf("[trace] 无效的 X-Trace-ID %q，回退到自动生成: %v", customID, err)
				ctx = prop.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))
			} else if tid.IsValid() {
				// 构造一个以自定义 TraceID 为根的新 SpanContext
				sc := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    tid,
					SpanID:     newSpanID(),
					TraceFlags: trace.FlagsSampled,
					Remote:     false,
				})
				ctx = trace.ContextWithSpanContext(ctx, sc)
			} else {
				log.Printf("[trace] X-Trace-ID %q 为全零，回退到自动生成", customID)
				ctx = prop.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))
			}
		} else {
			// 无自定义 ID：从标准 traceparent / tracestate header 提取上游 SpanContext
			ctx = prop.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))
		}

		ctx, span := tracer.Start(ctx, c.FullPath())
		defer span.End()

		// 响应头始终回传当前 TraceID
		// 兜底：OTel SDK 未配置时（如测试环境）TraceID 可能为全零，此时生成随机 ID
		traceID := span.SpanContext().TraceID().String()
		if traceID == "00000000000000000000000000000000" {
			traceID = newRandID()
		}
		c.Header(traceIDHeader, traceID)
		c.Set(CtxTraceID, traceID)

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// newSpanID 生成一个随机 SpanID（8 字节）。
func newSpanID() trace.SpanID {
	var sid trace.SpanID
	_, _ = rand.Read(sid[:])
	return sid
}

// newRandID 保留向后兼容：生成 16 字节随机 hex 字符串（用于旧 TraceID 兜底）。
func newRandID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}