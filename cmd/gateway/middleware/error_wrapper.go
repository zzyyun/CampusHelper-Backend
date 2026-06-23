package middleware

import (
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"

	"go_projects/praProject1/pkg/contextx"
	"go_projects/praProject1/pkg/errcode"
)

// CtxTraceID 是 Trace 中间件在 gin.Context 上注册的 trace_id 键名。
// Gateway 的 Trace 中间件调用 c.Set("trace_id", traceID)，本中间件复用同一 key。
const CtxTraceID = "trace_id"

// errorBody 是统一错误响应体的内部结构，对外序列化为 {code, message, trace_id}。
type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

// ErrorResponse 写入统一格式错误响应 {code, message, trace_id} 并中止后续 handler。
//
// 行为：
//   - 自动从 gin.Context 提取 trace_id（由 Trace 中间件注入）
//   - HTTP 状态码由业务码通过 errcode.HTTPStatus 推导
//   - 调用 c.Abort，确保后续 handler 不会再被调用
func ErrorResponse(c *gin.Context, bizCode int, message string) {
	traceID := extractTraceID(c)
	body := errorBody{
		Code:    bizCode,
		Message: message,
		TraceID: traceID,
	}
	c.AbortWithStatusJSON(errcode.HTTPStatus(bizCode), body)
}

// GRPCErrorResponse 把 gRPC 调用错误转换为统一格式错误响应。
//
// 行为：
//   - 使用 status.Code(err) 提取 gRPC 标准 Code
//   - 通过 errcode.FromGRPC 转换为业务错误码 + HTTP 状态码
//   - message 默认使用 gRPC 错误的描述（status.Convert(err).Message()）
//   - 当 err 为 nil 时，按 Internal 错误处理（防御性兜底）
func GRPCErrorResponse(c *gin.Context, err error) {
	if err == nil {
		ErrorResponse(c, errcode.ErrInternal, "下游调用异常")
		return
	}
	st, _ := status.FromError(err)
	bizCode, _ := errcode.FromGRPC(st.Code())
	ErrorResponse(c, bizCode, st.Message())
}

// extractTraceID 优先从 gin.Context 读取 trace_id，缺失时回退到 contextx / OTel context。
//
// 读取顺序：
//  1. gin.Context 的 CtxTraceID（Trace 中间件通过 c.Set 注入）
//  2. contextx.GetTraceID（兼容其他中间件直接写入 context 的场景）
//  3. 空字符串（响应中 trace_id 为空，前端可据此识别"无追踪"）
func extractTraceID(c *gin.Context) string {
	if c != nil {
		if v, ok := c.Get(CtxTraceID); ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if c != nil && c.Request != nil {
		if id := contextx.GetTraceID(c.Request.Context()); id != "" {
			return id
		}
	}
	return ""
}