package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go_projects/praProject1/pkg/errcode"
)

func init() {
	// 测试期间关闭 Gin 的 debug 日志
	gin.SetMode(gin.TestMode)
}

// runMiddleware 构造一个仅挂载 ErrorResponse 中间件 + 终止 handler 的 gin.Engine，
// 执行请求并返回响应记录。
func runMiddleware(t *testing.T, bizCode int, message string, withTraceID string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if withTraceID != "" {
			c.Set(CtxTraceID, withTraceID)
		}
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		ErrorResponse(c, bizCode, message)
	})
	// 此 handler 不应被触发（ErrorResponse 已 Abort）
	r.GET("/x", func(c *gin.Context) {
		t.Errorf("downstream handler should not be called after ErrorResponse")
		c.String(http.StatusOK, "should-not-reach")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestErrorResponse_BodyAndStatus 验证响应体包含 code/message/trace_id 三字段，HTTP 状态由 code 推导。
func TestErrorResponse_BodyAndStatus(t *testing.T) {
	w := runMiddleware(t, errcode.ErrMissingToken, "缺少 token", "trace-abc-123")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("HTTP status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var got errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not valid JSON: %v, body=%s", err, w.Body.String())
	}
	if got.Code != errcode.ErrMissingToken {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrMissingToken)
	}
	if got.Message != "缺少 token" {
		t.Errorf("message = %q, want %q", got.Message, "缺少 token")
	}
	if got.TraceID != "trace-abc-123" {
		t.Errorf("trace_id = %q, want %q", got.TraceID, "trace-abc-123")
	}
}

// TestErrorResponse_AllowsDownstream 测试：当 handler 不调用 ErrorResponse 时，链路正常通过。
// 这是为了保证 Abort 语义只在显式调用时生效。
func TestErrorResponse_AllowsDownstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/y", func(c *gin.Context) {
		c.Set(CtxTraceID, "trace-y")
		// 不调用 ErrorResponse，handler 正常返回
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	req := httptest.NewRequest(http.MethodGet, "/y", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("HTTP status = %d, want 200", w.Code)
	}
}

// TestErrorResponse_MissingTraceID 验证未注入 trace_id 时响应体 trace_id 字段为空字符串（不报错）。
func TestErrorResponse_MissingTraceID(t *testing.T) {
	w := runMiddleware(t, errcode.ErrRateLimited, "请求过于频繁", "")

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("HTTP status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	var got errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if got.TraceID != "" {
		t.Errorf("trace_id = %q, want empty", got.TraceID)
	}
	if got.Code != errcode.ErrRateLimited {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrRateLimited)
	}
}

// TestErrorResponse_HTTPMapping 验证不同业务码的 HTTP 状态码映射。
func TestErrorResponse_HTTPMapping(t *testing.T) {
	cases := []struct {
		name       string
		bizCode    int
		wantStatus int
	}{
		{"401 missing token", errcode.ErrMissingToken, http.StatusUnauthorized},
		{"403 campus not bound", errcode.ErrCampusNotBound, http.StatusForbidden},
		{"429 rate limited", errcode.ErrRateLimited, http.StatusTooManyRequests},
		{"400 invalid param", errcode.ErrInvalidParam, http.StatusBadRequest},
		{"502 downstream unavailable", errcode.ErrDownstreamUnavailable, http.StatusBadGateway},
		{"500 internal", errcode.ErrInternal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := runMiddleware(t, tc.bizCode, "test", "trace")
			if w.Code != tc.wantStatus {
				t.Errorf("HTTP status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}

// TestGRPCErrorResponse 验证 gRPC 错误转换为统一错误响应。
func TestGRPCErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/grpc", func(c *gin.Context) {
		c.Set(CtxTraceID, "trace-grpc")
		GRPCErrorResponse(c, status.Error(codes.Unauthenticated, "token expired"))
	})
	req := httptest.NewRequest(http.MethodGet, "/grpc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("HTTP status = %d, want 401", w.Code)
	}
	var got errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Code != errcode.ErrInvalidToken {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrInvalidToken)
	}
	if got.Message != "token expired" {
		t.Errorf("message = %q, want %q", got.Message, "token expired")
	}
	if got.TraceID != "trace-grpc" {
		t.Errorf("trace_id = %q, want %q", got.TraceID, "trace-grpc")
	}
}

// TestGRPCErrorResponse_Nil 验证传入 nil err 时不会 panic，落到 Internal 错误。
func TestGRPCErrorResponse_Nil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/grpc-nil", func(c *gin.Context) {
		GRPCErrorResponse(c, nil)
	})
	req := httptest.NewRequest(http.MethodGet, "/grpc-nil", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP status = %d, want 500", w.Code)
	}
	var got errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Code != errcode.ErrInternal {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrInternal)
	}
}

// TestGRPCErrorResponse_Mapping 验证多个 gRPC Code 正确转换。
func TestGRPCErrorResponse_Mapping(t *testing.T) {
	cases := []struct {
		name       string
		grpcCode   codes.Code
		grpcMsg    string
		wantCode   int
		wantStatus int
	}{
		{"permission denied", codes.PermissionDenied, "forbidden", errcode.ErrInsufficientPermission, http.StatusForbidden},
		{"not found", codes.NotFound, "no such", errcode.ErrDownstreamNotFound, http.StatusNotFound},
		{"unavailable", codes.Unavailable, "down", errcode.ErrDownstreamUnavailable, http.StatusBadGateway},
		{"deadline exceeded", codes.DeadlineExceeded, "slow", errcode.ErrDownstreamTimeout, http.StatusGatewayTimeout},
		{"resource exhausted", codes.ResourceExhausted, "throttled", errcode.ErrRateLimited, http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/g", func(c *gin.Context) {
				GRPCErrorResponse(c, status.Error(tc.grpcCode, tc.grpcMsg))
			})
			req := httptest.NewRequest(http.MethodGet, "/g", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("HTTP status = %d, want %d", w.Code, tc.wantStatus)
			}
			var got errorBody
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			if got.Code != tc.wantCode {
				t.Errorf("code = %d, want %d", got.Code, tc.wantCode)
			}
			if got.Message != tc.grpcMsg {
				t.Errorf("message = %q, want %q", got.Message, tc.grpcMsg)
			}
		})
	}
}