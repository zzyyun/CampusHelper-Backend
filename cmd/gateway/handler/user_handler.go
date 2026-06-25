package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	user_pb "go_projects/praProject1/PB/pb/user_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

type wxLoginReq struct {
	Code string `json:"code" binding:"required"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type bindCampusReq struct {
	SchoolName string `json:"school_name" binding:"required,min=1"`
}

// updateUserInfoReq 修改昵称/头像的请求体；两个字段均为可选，至少传一个。
type updateUserInfoReq struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// ─── POST /api/v1/user/login ─────────────────────────────────────────────────

// WxLogin 微信登录入口。错误响应走统一格式（gRPC Code → 业务码）。
//
// 返回 access_token + refresh_token；前端应在 access 过期后用 refresh
// 调 POST /user/refresh 续签。
func WxLogin(c *gin.Context) {
	var req wxLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	resp, err := client.UserClient.WxLogin(baseCtx(c), &user_pb.WxLoginRequest{JsCode: req.Code})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":    resp.AccessToken,
		"refresh_token":   resp.RefreshToken,
		"is_bound_campus": resp.IsBoundCampus,
		"school_id":       resp.SchoolId,
	})
}

// ─── POST /api/v1/user/refresh  (白名单：无 JWT) ────────────────────────────

// RefreshToken 用 refresh_token 换取新的 access_token。
//
// 错误约定（覆盖 errcode.FromGRPC 的默认映射，让前端能区分处理）：
//   - Unauthenticated + 消息含 "expired" → 20004 refresh token 过期
//   - Unauthenticated + 其他            → 20005 refresh token 无效
//   - NotFound                          → 20006 + message（用户不存在）
func RefreshToken(c *gin.Context) {
	var req refreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	resp, err := client.UserClient.RefreshToken(baseCtx(c), &user_pb.RefreshTokenRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.Unauthenticated:
			// 在 errcode 通用映射基础上细分"过期"与"非法"
			if errors.Is(err, status.Error(codes.Unauthenticated, "refresh token expired")) ||
				(st.Message() == "refresh token expired") {
				middleware.ErrorResponse(c, errcode.ErrRefreshTokenExpired, st.Message())
				return
			}
			middleware.ErrorResponse(c, errcode.ErrRefreshTokenInvalid, st.Message())
			return
		default:
			middleware.GRPCErrorResponse(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":    resp.AccessToken,
		"is_bound_campus": resp.IsBoundCampus,
		"school_id":       resp.SchoolId,
	})
}

// ─── PUT /api/v1/user/campus  (JWT) ───────────

// BindCampus 用户绑定学校。
func BindCampus(c *gin.Context) {
	var req bindCampusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.UserClient.BindCampus(ctx, &user_pb.BindCampusRequest{
		SchoolName: req.SchoolName,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	if resp.Code != 0 {
		c.JSON(errcode.HTTPStatus(int(resp.Code)), gin.H{
			"code":     resp.Code,
			"message":  resp.Message,
			"trace_id": c.GetString(middleware.CtxTraceID),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ─── GET /api/v1/user/me  (JWT) ──────────────────────────────────────────────

// GetCurrentUser 获取当前登录用户信息。
func GetCurrentUser(c *gin.Context) {
	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.UserClient.GetCurrentUser(ctx, &user_pb.GetCurrentUserRequest{})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── PUT /api/v1/user/info  (JWT) ────────────────────────────────────────────

// UpdateUserInfo 更新昵称 / 头像。
func UpdateUserInfo(c *gin.Context) {
	var req updateUserInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	if req.Nickname == "" && req.AvatarURL == "" {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "至少需要更新昵称或头像")
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.UserClient.UpdateUserInfo(ctx, &user_pb.UpdateUserInfoRequest{
		Nickname:  req.Nickname,
		AvatarUrl: req.AvatarURL,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	if resp.Code != 0 {
		c.JSON(errcode.HTTPStatus(int(resp.Code)), gin.H{
			"code":     resp.Code,
			"message":  resp.Message,
			"trace_id": c.GetString(middleware.CtxTraceID),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}


// ─── GET /api/v1/schools ─────────────────────────────────────────────────────

type listSchoolsReq struct {
	Keyword  string `form:"keyword"`
	PageSize int32  `form:"page_size"`
	Cursor   string `form:"cursor"`
}

// ListSchools 搜索/列出学校。JWT 鉴权，不强绑学校。
func ListSchools(c *gin.Context) {
	var req listSchoolsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}
	ctx, ok := authCtx(c)
	if !ok {
		return
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	resp, err := client.UserClient.ListSchools(ctx, &user_pb.ListSchoolsRequest{
		Keyword:  req.Keyword,
		PageSize: pageSize,
		Cursor:   req.Cursor,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── context helpers ─────────────────────────────────────────────────────────

// baseCtx 提取请求 context（携带 OTel trace 上下文）。
func baseCtx(c *gin.Context) context.Context {
	return c.Request.Context()
}

// authCtx 转发 JWT 解析出的用户身份到下游 gRPC metadata。
//
// 注入三个 key（多租户隔离 + 鉴权最小集）：
//   - user-id   : int64
//   - user-role : int8
//   - school-id : int64（依赖 Issue #18，0 表示未绑定）
//
// 返回 ok=false 时表示身份缺失或格式异常，此时已写入统一错误响应。
func authCtx(c *gin.Context) (context.Context, bool) {
	v, exists := c.Get(middleware.CtxUserID)
	if !exists {
		middleware.ErrorResponse(c, errcode.ErrMissingToken, "缺少用户身份")
		return nil, false
	}
	uid, ok := v.(int64)
	if !ok {
		middleware.ErrorResponse(c, errcode.ErrInvalidToken, "未认证")
		return nil, false
	}

	role, _ := c.Get(middleware.CtxRole)
	r, _ := role.(int8)

	// Issue #19：从 gin.Context 读取 school_id 并注入 metadata。
	// school_id = 0 表示未绑定；调用方若使用 RequireSchoolBound 中间件，
	// 写接口会在此处之前已被拒绝，命中此处的请求多为读接口。
	schoolID, _ := c.Get(middleware.CtxSchoolID)
	sid, _ := schoolID.(int64)

	md := metadata.Pairs(
		"user-id", strconv.FormatInt(uid, 10),
		"user-role", strconv.FormatInt(int64(r), 10),
		"school-id", strconv.FormatInt(sid, 10),
	)
	return metadata.NewOutgoingContext(c.Request.Context(), md), true
}