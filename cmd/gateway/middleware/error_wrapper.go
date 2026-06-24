package middleware

import (
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/status"

	"go_projects/praProject1/pkg/contextx"
	"go_projects/praProject1/pkg/errcode"
)

// CtxTraceID 是 Trace 中间件在 gin.Context 上注册的 trace_id 键名。
const CtxTraceID = "trace_id"

// CtxSchoolID 是 JWTAuth 中间件在 gin.Context 上注册的 school_id 键名（int64）。
const CtxSchoolID = "user_school_id"

// errorBody 是统一错误响应体的内部结构，对外序列化为 {code, message, trace_id}。
type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

// ErrorResponse 写入统一格式错误响应 {code, message, trace_id} 并中止后续 handler。
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
func GRPCErrorResponse(c *gin.Context, err error) {
	if err == nil {
		ErrorResponse(c, errcode.ErrInternal, "下游调用异常")
		return
	}
	st, _ := status.FromError(err)
	bizCode, _ := errcode.FromGRPC(st.Code())
	ErrorResponse(c, bizCode, st.Message())
}

// extractTraceID 优先从 gin.Context 读取 trace_id，缺失时回退到 contextx。
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