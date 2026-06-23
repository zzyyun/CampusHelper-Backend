package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"

	user_pb "go_projects/praProject1/PB/pb/user_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

type wxLoginReq struct {
	Code string `json:"code" binding:"required"`
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
		return // authCtx 已写入 401 响应
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
// 修改当前登录用户的昵称和/或头像。用户身份由 JWT 决定，禁止修改他人资料。

// UpdateUserInfo 更新昵称 / 头像。
func UpdateUserInfo(c *gin.Context) {
	var req updateUserInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	// 至少需要更新一个字段
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

// ─── context helpers ─────────────────────────────────────────────────────────

// baseCtx 提取请求 context（携带 OTel trace 上下文）。
func baseCtx(c *gin.Context) context.Context {
	return c.Request.Context()
}

// authCtx 转发 JWT 解析出的用户身份到下游 gRPC metadata。
//
// 返回 ok=false 时表示身份缺失或格式异常，此时已写入 401 响应，调用方必须
// 直接 return，不得继续发起 gRPC 调用。这样"身份丢失"会在网关层就暴露，
// 而不是带着空 context 打到下游、被一个含糊的"未认证"掩盖。
func authCtx(c *gin.Context) (context.Context, bool) {
	v, exists := c.Get(middleware.CtxUserID)
	if !exists {
		// JWTAuth 中间件未执行或认证失败
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

	md := metadata.Pairs(
		"user-id", strconv.FormatInt(uid, 10),
		"user-role", strconv.FormatInt(int64(r), 10),
	)
	return metadata.NewOutgoingContext(c.Request.Context(), md), true
}