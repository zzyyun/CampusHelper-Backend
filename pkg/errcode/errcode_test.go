package errcode

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestHTTPStatus(t *testing.T) {
	cases := []struct {
		name string
		code int
		want int
	}{
		{"success", Success, 200},
		{"third-party default", ErrWechatInvalidCode, 502},
		{"auth: missing token", ErrMissingToken, 401},
		{"auth: token expired", ErrTokenExpired, 401},
		{"auth: invalid token", ErrInvalidToken, 401},
		{"auth: refresh expired", ErrRefreshTokenExpired, 401},
		{"auth: refresh invalid", ErrRefreshTokenInvalid, 401},
		{"auth: campus not bound", ErrCampusNotBound, 403},
		{"auth: insufficient permission", ErrInsufficientPermission, 403},
		{"auth: upper bound 29999", 29999, 401},
		{"rate limit", ErrRateLimited, 429},
		{"param error", ErrInvalidParam, 400},
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
			if got := HTTPStatus(tc.code); got != tc.want {
				t.Errorf("HTTPStatus(%d) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

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