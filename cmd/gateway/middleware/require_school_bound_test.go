package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go_projects/praProject1/pkg/errcode"
)

// TestRequireSchoolBound_NotBound 验证未绑定学校用户（schoolID=0）返回 403。
func TestRequireSchoolBound_NotBound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(CtxSchoolID, int64(0)) // 模拟未绑定
		c.Next()
	})
	r.Use(RequireSchoolBound())
	r.GET("/x", func(c *gin.Context) {
		t.Errorf("handler should not be called for unbound user")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("HTTP = %d, want 403", w.Code)
	}
	var got errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Code != errcode.ErrCampusNotBound {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrCampusNotBound)
	}
	if got.Message == "" {
		t.Error("message should not be empty")
	}
}

// TestRequireSchoolBound_Bound 验证已绑定用户正常通过。
func TestRequireSchoolBound_Bound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(CtxSchoolID, int64(12345))
		c.Next()
	})
	r.Use(RequireSchoolBound())
	downstreamCalled := false
	r.GET("/x", func(c *gin.Context) {
		downstreamCalled = true
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HTTP = %d, want 200", w.Code)
	}
	if !downstreamCalled {
		t.Error("downstream handler should be called for bound user")
	}
}

// TestRequireSchoolBound_MissingCtx 验证 CtxSchoolID 未设置时返回 401。
func TestRequireSchoolBound_MissingCtx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 故意不设置 CtxSchoolID
	r.Use(RequireSchoolBound())
	r.GET("/x", func(c *gin.Context) {
		t.Errorf("handler should not be called when CtxSchoolID missing")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("HTTP = %d, want 401", w.Code)
	}
	var got errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Code != errcode.ErrMissingToken {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrMissingToken)
	}
}

// TestRequireSchoolBound_WrongType 验证 CtxSchoolID 类型错误时返回 401。
func TestRequireSchoolBound_WrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(CtxSchoolID, "not-an-int64") // 错误类型
		c.Next()
	})
	r.Use(RequireSchoolBound())
	r.GET("/x", func(c *gin.Context) {
		t.Errorf("handler should not be called for wrong type")
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("HTTP = %d, want 401", w.Code)
	}
	var got errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Code != errcode.ErrInvalidToken {
		t.Errorf("code = %d, want %d", got.Code, errcode.ErrInvalidToken)
	}
}

// TestRequireSchoolBound_TraceIDPropagated 验证错误响应包含 trace_id。
func TestRequireSchoolBound_TraceIDPropagated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(CtxTraceID, "trace-school-test")
		c.Set(CtxSchoolID, int64(0))
		c.Next()
	})
	r.Use(RequireSchoolBound())
	r.GET("/x", func(c *gin.Context) {})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var got errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.TraceID != "trace-school-test" {
		t.Errorf("trace_id = %q, want %q", got.TraceID, "trace-school-test")
	}
}