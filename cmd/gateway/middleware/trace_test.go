package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
)

// traceIDPattern 匹配 32 字符 hex 格式的合法 TraceID。
var traceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// TestTrace_WithCustomTraceID 验证请求头携带合法 X-Trace-ID 时响应头原样回传。
func TestTrace_WithCustomTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Trace())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	customID := "abcdef0123456789abcdef0123456789" // 32 字符 hex
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(traceIDHeader, customID)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200", w.Code)
	}

	respID := w.Header().Get(traceIDHeader)
	if respID != customID {
		t.Errorf("X-Trace-ID = %q, want %q", respID, customID)
	}

	// 验证 trace_id 已注入 gin.Context
	// （通过错误中间件间接验证，这里检查 header 格式即可）
	if !traceIDPattern.MatchString(respID) {
		t.Errorf("响应 X-Trace-ID %q 不是合法 32 字符 hex", respID)
	}
}

// TestTrace_WithoutCustomTraceID 验证无 X-Trace-ID 时自动生成非零 TraceID。
func TestTrace_WithoutCustomTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Trace())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// 不设置 X-Trace-ID

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	respID := w.Header().Get(traceIDHeader)
	if !traceIDPattern.MatchString(respID) {
		t.Errorf("响应 X-Trace-ID %q 不是合法 32 字符 hex", respID)
	}
	if respID == "00000000000000000000000000000000" {
		t.Error("自动生成的 TraceID 不应为全零")
	}
}

// TestTrace_InvalidCustomTraceID 验证非法 X-Trace-ID 降级到自动生成。
func TestTrace_InvalidCustomTraceID(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{"长度不足", "abc123"},
		{"非 hex 字符", "gggg000000000000gggg000000000000"},
		{"全零", "00000000000000000000000000000000"},
		{"空字符串（通过不设 header 测试）", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(Trace())
			r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.id != "" {
				req.Header.Set(traceIDHeader, tc.id)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			respID := w.Header().Get(traceIDHeader)
			if !traceIDPattern.MatchString(respID) {
				t.Errorf("响应 X-Trace-ID %q 不是合法 32 字符 hex", respID)
			}
			// 非法输入不应被回传
			if respID == tc.id && tc.id != "" {
				t.Errorf("非法 X-Trace-ID %q 不应被回传", tc.id)
			}
			if respID == "00000000000000000000000000000000" {
				t.Error("回退生成的 TraceID 不应为全零")
			}
		})
	}
}

// TestTrace_TraceIDInGinContext 验证 trace_id 注入到了 gin.Context。
func TestTrace_TraceIDInGinContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Trace())
	r.GET("/test", func(c *gin.Context) {
		v, ok := c.Get(CtxTraceID)
		if !ok {
			t.Error("gin.Context 中缺少 trace_id")
			return
		}
		tid, ok := v.(string)
		if !ok || !traceIDPattern.MatchString(tid) {
			t.Errorf("trace_id 类型或格式异常: %v", v)
		}
		c.String(200, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(traceIDHeader, "aaaaaaaa00000000bbbbbbbb11111111")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200", w.Code)
	}
}