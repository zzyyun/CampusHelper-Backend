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
	gin.SetMode(gin.TestMode)
}

func runErrorMiddleware(t *testing.T, bizCode int, message string, withTraceID string) *httptest.ResponseRecorder {
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
	r.GET("/x", func(c *gin.Context) {
		t.Errorf("downstream handler should not be called after ErrorResponse")
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestErrorResponse_BodyAndStatus(t *testing.T) {
	w := runErrorMiddleware(t, errcode.ErrMissingToken, "缺少 token", "trace-abc-123")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("HTTP status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	var got errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body invalid: %v", err)
	}
	if got.Code != errcode.ErrMissingToken || got.Message != "缺少 token" || got.TraceID != "trace-abc-123" {
		t.Errorf("body = %+v", got)
	}
}

func TestErrorResponse_HTTPMapping(t *testing.T) {
	cases := []struct {
		bizCode    int
		wantStatus int
	}{
		{errcode.ErrMissingToken, http.StatusUnauthorized},
		{errcode.ErrCampusNotBound, http.StatusForbidden},
		{errcode.ErrRateLimited, http.StatusTooManyRequests},
		{errcode.ErrInvalidParam, http.StatusBadRequest},
		{errcode.ErrDownstreamUnavailable, http.StatusBadGateway},
		{errcode.ErrInternal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		w := runErrorMiddleware(t, tc.bizCode, "test", "trace")
		if w.Code != tc.wantStatus {
			t.Errorf("biz=%d HTTP = %d, want %d", tc.bizCode, w.Code, tc.wantStatus)
		}
	}
}

func TestGRPCErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/g", func(c *gin.Context) {
		c.Set(CtxTraceID, "trace-grpc")
		GRPCErrorResponse(c, status.Error(codes.Unauthenticated, "token expired"))
	})
	req := httptest.NewRequest(http.MethodGet, "/g", nil)
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

func TestGRPCErrorResponse_Nil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/g", func(c *gin.Context) {
		GRPCErrorResponse(c, nil)
	})
	req := httptest.NewRequest(http.MethodGet, "/g", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP = %d, want 500", w.Code)
	}
}