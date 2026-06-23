package errcode

import (
	"testing"

	"google.golang.org/grpc/codes"
)

// TestHTTPStatus 验证各段位错误码的 HTTP 状态码映射。
func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		code int
		want int
	}{
		{"success", Success, 200},
		{"third-party default", ErrWechatInvalidCode, 502},
		{"third-party upper bound", 19999, 502},
		{"auth: missing token", ErrMissingToken, 401},
		{"auth: token expired", ErrTokenExpired, 401},
		{"auth: invalid token", ErrInvalidToken, 401},
		{"auth: refresh expired", ErrRefreshTokenExpired, 401},
		{"auth: refresh invalid", ErrRefreshTokenInvalid, 401},
		{"auth: campus not bound", ErrCampusNotBound, 403},
		{"auth: insufficient permission", ErrInsufficientPermission, 403},
		{"auth: upper bound 29999", 29999, 401},
		{"rate limit", ErrRateLimited, 429},
		{"rate limit upper bound", 39999, 429},
		{"param error", ErrInvalidParam, 400},
		{"param upper bound", 49999, 400},
		{"downstream unavailable", ErrDownstreamUnavailable, 502},
		{"downstream timeout", ErrDownstreamTimeout, 504},
		{"downstream internal", ErrDownstreamInternal, 500},
		{"downstream invalid arg", ErrDownstreamInvalidArgument, 400},
		{"downstream not found", ErrDownstreamNotFound, 404},
		{"downstream upper bound", 59999, 502},
		{"system error", ErrInternal, 500},
		{"system upper bound", 99999, 500},
		{"unknown segment 60000", 60000, 500},
		{"negative code", -1, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HTTPStatus(tc.code)
			if got != tc.want {
				t.Errorf("HTTPStatus(%d) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

// TestFromGRPC 验证 gRPC Code 到业务错误码 + HTTP Status 的映射。
func TestFromGRPC(t *testing.T) {
	cases := []struct {
		name       string
		grpcCode   codes.Code
		wantBiz    int
		wantStatus int
	}{
		{"OK", codes.OK, Success, 200},
		{"Canceled", codes.Canceled, ErrInternal, 500},
		{"Unknown", codes.Unknown, ErrInternal, 500},
		{"InvalidArgument", codes.InvalidArgument, ErrDownstreamInvalidArgument, 400},
		{"DeadlineExceeded", codes.DeadlineExceeded, ErrDownstreamTimeout, 504},
		{"NotFound", codes.NotFound, ErrDownstreamNotFound, 404},
		{"AlreadyExists", codes.AlreadyExists, ErrInvalidParam, 400},
		{"PermissionDenied", codes.PermissionDenied, ErrInsufficientPermission, 403},
		{"ResourceExhausted", codes.ResourceExhausted, ErrRateLimited, 429},
		{"FailedPrecondition", codes.FailedPrecondition, ErrInvalidParam, 400},
		{"Aborted", codes.Aborted, ErrInternal, 500},
		{"OutOfRange", codes.OutOfRange, ErrInvalidParam, 400},
		{"Unimplemented", codes.Unimplemented, ErrDownstreamInternal, 500},
		{"Internal", codes.Internal, ErrDownstreamInternal, 500},
		{"Unavailable", codes.Unavailable, ErrDownstreamUnavailable, 502},
		{"DataLoss", codes.DataLoss, ErrInternal, 500},
		{"Unauthenticated", codes.Unauthenticated, ErrInvalidToken, 401},
		{"unknown code", codes.Code(9999), ErrInternal, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBiz, gotStatus := FromGRPC(tc.grpcCode)
			if gotBiz != tc.wantBiz {
				t.Errorf("FromGRPC(%v) bizCode = %d, want %d", tc.grpcCode, gotBiz, tc.wantBiz)
			}
			if gotStatus != tc.wantStatus {
				t.Errorf("FromGRPC(%v) httpStatus = %d, want %d", tc.grpcCode, gotStatus, tc.wantStatus)
			}
		})
	}
}

// TestConstantsDistinct 验证关键错误码常量互不相同，防止粘贴错误。
func TestConstantsDistinct(t *testing.T) {
	all := map[string]int{
		"Success":                     Success,
		"ErrWechatInvalidCode":        ErrWechatInvalidCode,
		"ErrWechatUnavailable":        ErrWechatUnavailable,
		"ErrMissingToken":             ErrMissingToken,
		"ErrTokenExpired":             ErrTokenExpired,
		"ErrInvalidToken":             ErrInvalidToken,
		"ErrRefreshTokenExpired":      ErrRefreshTokenExpired,
		"ErrRefreshTokenInvalid":      ErrRefreshTokenInvalid,
		"ErrCampusNotBound":           ErrCampusNotBound,
		"ErrInsufficientPermission":   ErrInsufficientPermission,
		"ErrRateLimited":              ErrRateLimited,
		"ErrInvalidParam":             ErrInvalidParam,
		"ErrDownstreamUnavailable":    ErrDownstreamUnavailable,
		"ErrDownstreamTimeout":        ErrDownstreamTimeout,
		"ErrDownstreamInternal":       ErrDownstreamInternal,
		"ErrDownstreamInvalidArgument": ErrDownstreamInvalidArgument,
		"ErrDownstreamNotFound":       ErrDownstreamNotFound,
		"ErrInternal":                 ErrInternal,
	}
	seen := make(map[int]string)
	for name, code := range all {
		if existing, ok := seen[code]; ok {
			t.Errorf("constant %s and %s share the same value %d", name, existing, code)
		}
		seen[code] = name
	}
}