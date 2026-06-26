package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	message_pb "go_projects/praProject1/PB/pb/message_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── DTOs ───────────────────────────────────────────────────────────────────

type listNotificationsReq struct {
	Cursor   string `form:"cursor"`
	PageSize int32  `form:"page_size"`
	Type     string `form:"type"`
}

// ─── GET /api/v1/notifications ──────────────────────────────────────────────

// ListNotifications 获取当前用户的通知列表。
func ListNotifications(c *gin.Context) {
	var req listNotificationsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, uid, sid, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	resp, err := client.MessageClient.ListNotifications(ctx, &message_pb.ListNotificationsRequest{
		SchoolId: sid,
		UserId:   uid,
		Cursor:   req.Cursor,
		PageSize: pageSize,
		Type:     req.Type,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─── GET /api/v1/notifications/unread-count ─────────────────────────────────

// UnreadCount 获取当前用户未读通知数。
func UnreadCount(c *gin.Context) {
	ctx, uid, sid, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.MessageClient.UnreadCount(ctx, &message_pb.UnreadCountRequest{
		SchoolId: sid,
		UserId:   uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─── PUT /api/v1/notifications/:id/read ─────────────────────────────────────

// MarkRead 标记单条通知为已读。
func MarkRead(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的通知 ID")
		return
	}

	ctx, uid, _, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.MessageClient.MarkRead(ctx, &message_pb.MarkReadRequest{
		Id:     id,
		UserId: uid,
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

// ─── PUT /api/v1/notifications/read-all ─────────────────────────────────────

// MarkAllRead 标记全部通知为已读。
func MarkAllRead(c *gin.Context) {
	ctx, uid, sid, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.MessageClient.MarkAllRead(ctx, &message_pb.MarkAllReadRequest{
		SchoolId: sid,
		UserId:   uid,
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

	c.JSON(http.StatusOK, gin.H{"message": resp.Message})
}

// ─── DELETE /api/v1/notifications/:id ───────────────────────────────────────

// DeleteNotification 删除单条通知。
func DeleteNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的通知 ID")
		return
	}

	ctx, uid, _, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.MessageClient.DeleteNotification(ctx, &message_pb.DeleteNotificationRequest{
		Id:     id,
		UserId: uid,
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
