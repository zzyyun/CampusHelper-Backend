package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	file_pb "go_projects/praProject1/PB/pb/file_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── POST /api/v1/files/upload ─────────────────────────────────────────────

// UploadFile 上传单张图片（通过 multipart/form-data → gRPC 转发到 File Service）。
func UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "缺少文件字段 file")
		return
	}
	defer file.Close()

	category := c.DefaultPostForm("category", "other")

	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	// 读取文件字节
	data, err := io.ReadAll(file)
	if err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "读取文件失败")
		return
	}

	contentType := header.Header.Get("Content-Type")

	resp, err := client.FileClient.Upload(ctx, &file_pb.UploadRequest{
		SchoolId:    sid,
		UserId:      uid,
		Category:    category,
		Data:        data,
		Filename:    header.Filename,
		ContentType: contentType,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── GET /api/v1/files/:id ─────────────────────────────────────────────────

// GetFile 获取文件元数据。
func GetFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的文件 ID")
		return
	}

	ctx, uid, _, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.FileClient.GetFile(ctx, &file_pb.GetFileRequest{
		FileId: id,
		UserId: uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── DELETE /api/v1/files/:id ──────────────────────────────────────────────

// DeleteFile 删除文件（软删除）。
func DeleteFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的文件 ID")
		return
	}

	ctx, uid, _, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.FileClient.DeleteFile(ctx, &file_pb.DeleteFileRequest{
		FileId: id,
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
