// Package errcode 提供统一的业务错误码定义与 HTTP/gRPC 错误转换。
//
// 设计目标：
//   - 定义分段式错误码（0 成功 / 1xxxx 第三方 / 2xxxx 鉴权 / 3xxxx 限流 / 4xxxx 参数 / 5xxxx 下游 / 9xxxx 系统）
//   - 提供 HTTPStatus / FromGRPC 等转换函数，供 Gateway 统一错误响应中间件使用
//   - 错误码与 HTTP 状态码解耦：业务码用于前端逻辑判断，HTTP Status 用于网关/反向代理路由
package errcode

import (
	"google.golang.org/grpc/codes"
)

// 业务错误码常量定义
const (
	// Success 成功
	Success = 0

	// ── 1xxxx 第三方依赖错误 ─────────────────────────────────
	// ErrWechatInvalidCode 微信 code 无效或已过期
	ErrWechatInvalidCode = 10001
	// ErrWechatUnavailable 微信服务不可用
	ErrWechatUnavailable = 10002

	// ── 2xxxx 鉴权与权限错误 ─────────────────────────────────
	// ErrMissingToken 请求头缺少 Authorization 或格式错误
	ErrMissingToken = 20001
	// ErrTokenExpired Access Token 已过期
	ErrTokenExpired = 20002
	// ErrInvalidToken Access Token 签名无效或解析失败
	ErrInvalidToken = 20003
	// ErrRefreshTokenExpired Refresh Token 已过期
	ErrRefreshTokenExpired = 20004
	// ErrRefreshTokenInvalid Refresh Token 签名无效或解析失败
	ErrRefreshTokenInvalid = 20005
	// ErrCampusNotBound 用户未绑定学校，访问受保护写接口被拒
	ErrCampusNotBound = 20006
	// ErrInsufficientPermission 角色权限不足
	ErrInsufficientPermission = 20007
	// ErrUserBanned 用户已被封禁，禁止登录/刷新
	ErrUserBanned = 20008

	// ── 3xxxx 限流与配额错误 ─────────────────────────────────
	// ErrRateLimited 请求过于频繁，触发限流
	ErrRateLimited = 30001

	// ── 4xxxx 请求参数错误 ───────────────────────────────────
	// ErrInvalidParam 请求参数缺失或格式错误（兜底）
	ErrInvalidParam = 40001

	// ── 5xxxx 下游服务错误 ───────────────────────────────────
	// ErrDownstreamUnavailable 下游 gRPC 服务不可用
	ErrDownstreamUnavailable = 50001
	// ErrDownstreamTimeout 下游 gRPC 调用超时
	ErrDownstreamTimeout = 50002
	// ErrDownstreamInternal 下游 gRPC 返回 Internal 错误
	ErrDownstreamInternal = 50003
	// ErrDownstreamInvalidArgument 下游 gRPC 返回 InvalidArgument
	ErrDownstreamInvalidArgument = 50004
	// ErrDownstreamNotFound 下游资源不存在
	ErrDownstreamNotFound = 50005

	// ── 9xxxx 系统内部错误 ───────────────────────────────────
	// ErrInternal 系统内部错误（兜底）
	ErrInternal = 90001
)

// HTTPStatus 根据业务错误码返回对应的默认 HTTP 状态码。
func HTTPStatus(code int) int {
	switch {
	case code == Success:
		return 200
	case code >= 10000 && code < 20000:
		return 502
	case code >= 20000 && code < 30000:
		if code == ErrCampusNotBound || code == ErrInsufficientPermission || code == ErrUserBanned {
			return 403
		}
		return 401
	case code >= 30000 && code < 40000:
		return 429
	case code >= 40000 && code < 50000:
		return 400
	case code >= 50000 && code < 60000:
		switch code {
		case ErrDownstreamTimeout:
			return 504
		case ErrDownstreamNotFound:
			return 404
		case ErrDownstreamInvalidArgument:
			return 400
		case ErrDownstreamInternal:
			return 500
		default:
			return 502
		}
	case code >= 90000 && code < 100000:
		return 500
	default:
		return 500
	}
}

// FromGRPC 将 gRPC 标准 Code 转换为业务错误码 + 默认 HTTP 状态码。
func FromGRPC(code codes.Code) (bizCode int, httpStatus int) {
	switch code {
	case codes.OK:
		return Success, 200
	case codes.Canceled, codes.Unknown, codes.Aborted, codes.DataLoss:
		return ErrInternal, HTTPStatus(ErrInternal)
	case codes.InvalidArgument:
		return ErrDownstreamInvalidArgument, HTTPStatus(ErrDownstreamInvalidArgument)
	case codes.DeadlineExceeded:
		return ErrDownstreamTimeout, HTTPStatus(ErrDownstreamTimeout)
	case codes.NotFound:
		return ErrDownstreamNotFound, HTTPStatus(ErrDownstreamNotFound)
	case codes.AlreadyExists, codes.FailedPrecondition, codes.OutOfRange:
		return ErrInvalidParam, HTTPStatus(ErrInvalidParam)
	case codes.PermissionDenied:
		return ErrInsufficientPermission, HTTPStatus(ErrInsufficientPermission)
	case codes.ResourceExhausted:
		return ErrRateLimited, HTTPStatus(ErrRateLimited)
	case codes.Unimplemented, codes.Internal:
		return ErrDownstreamInternal, HTTPStatus(ErrDownstreamInternal)
	case codes.Unavailable:
		return ErrDownstreamUnavailable, HTTPStatus(ErrDownstreamUnavailable)
	case codes.Unauthenticated:
		return ErrInvalidToken, HTTPStatus(ErrInvalidToken)
	default:
		return ErrInternal, HTTPStatus(ErrInternal)
	}
}