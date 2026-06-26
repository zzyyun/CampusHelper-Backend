package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	user_pb "go_projects/praProject1/PB/pb/user_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

type adminBanUserReq struct {
	UserID int64  `json:"user_id" binding:"required,min=1"`
	Reason string `json:"reason"`
}

type adminUnbanUserReq struct {
	UserID int64 `json:"user_id" binding:"required,min=1"`
}

type adminSetRoleReq struct {
	TargetUserID int64 `json:"target_user_id" binding:"required,min=1"`
	Role         int32 `json:"role" binding:"required,min=1,max=2"`
}

type adminAuditContentReq struct {
	ContentID int64  `json:"content_id" binding:"required,min=1"`
	Action    string `json:"action" binding:"required,oneof=approve reject"`
	Reason    string `json:"reason"`
}

// ─── POST /api/v1/admin/users/ban ────────────────────────────────────────────

// AdminBanUser 管理员封禁用户。
func AdminBanUser(c *gin.Context) {
	var req adminBanUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, _, sid, ok := authCtxWithIDs(c) // uid 由 Gateway 注入 metadata，下游从 metadata 读取
	if !ok {
		return
	}

	resp, err := client.UserClient.BanUser(ctx, &user_pb.BanUserRequest{
		UserId:   req.UserID,
		SchoolId: sid,
		Reason:   req.Reason,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── POST /api/v1/admin/users/unban ──────────────────────────────────────────

// AdminUnbanUser 管理员解封用户。
func AdminUnbanUser(c *gin.Context) {
	var req adminUnbanUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, _, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.UserClient.UnbanUser(ctx, &user_pb.UnbanUserRequest{
		UserId:   req.UserID,
		SchoolId: sid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── GET /api/v1/admin/users/list ────────────────────────────────────────────

// AdminListUsers 管理员查询用户列表。
func AdminListUsers(c *gin.Context) {
	ctx, _, sid, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	schoolID, _ := strconv.ParseInt(c.DefaultQuery("school_id", "0"), 10, 64)
	role, _ := strconv.Atoi(c.DefaultQuery("role", "0"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	keyword := c.Query("keyword")
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	cursor := c.Query("cursor")

	// 管理员路由组已通过 RequireRole 校验，此处透传 school_id（admin 会被下游覆盖）
	_ = sid

	resp, err := client.UserClient.ListUsers(ctx, &user_pb.ListUsersRequest{
		SchoolId: schoolID,
		Role:     int32(role),
		Status:   int32(status),
		Keyword:  keyword,
		PageSize: int32(pageSize),
		Cursor:   cursor,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── POST /api/v1/admin/users/set-role ───────────────────────────────────────

// AdminSetUserRole 超级管理员设置用户角色。
func AdminSetUserRole(c *gin.Context) {
	var req adminSetRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, _, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.UserClient.SetUserRole(ctx, &user_pb.SetUserRoleRequest{
		TargetUserId: req.TargetUserID,
		Role:         req.Role,
		SchoolId:     sid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── GET /api/v1/admin/content/audit-list ────────────────────────────────────

// AdminListContentForAudit 管理员查看待审核内容列表。
func AdminListContentForAudit(c *gin.Context) {
	ctx, _, sid, ok := readCtxWithIDs(c)
	if !ok {
		return
	}
	_ = sid

	schoolID, _ := strconv.ParseInt(c.DefaultQuery("school_id", "0"), 10, 64)
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	cursor := c.Query("cursor")

	resp, err := client.UserClient.ListContentForAudit(ctx, &user_pb.ListContentForAuditRequest{
		SchoolId: schoolID,
		PageSize: int32(pageSize),
		Cursor:   cursor,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── POST /api/v1/admin/content/audit ────────────────────────────────────────

// AdminAuditContent 管理员审核内容（通过 or 驳回）。
func AdminAuditContent(c *gin.Context) {
	var req adminAuditContentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, _, _, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.UserClient.AuditContent(ctx, &user_pb.AuditContentRequest{
		ContentId: req.ContentID,
		Action:    req.Action,
		Reason:    req.Reason,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
