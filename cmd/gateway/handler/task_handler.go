package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	task_pb "go_projects/praProject1/PB/pb/task_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── DTOs ───────────────────────────────────────────────────────────────────

type createTaskReq struct {
	TaskType    int32  `json:"task_type" binding:"required,min=1,max=3"`
	Title       string `json:"title" binding:"required,min=1,max=128"`
	Description string `json:"description"`
	Location    string `json:"location"`
	RewardDesc  string `json:"reward_desc"`
	Contact     string `json:"contact"`
	Note        string `json:"note"`
	ExpiredAt   int64  `json:"expired_at"`
}

type updateTaskReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	RewardDesc  string `json:"reward_desc"`
	Contact     string `json:"contact"`
	Note        string `json:"note"`
}

type listTasksReq struct {
	Cursor   string `form:"cursor"`
	PageSize int32  `form:"page_size"`
	TaskType int32  `form:"task_type"`
}

type claimTaskReq struct {
	Contact string `json:"contact" binding:"required"`
	Message string `json:"message"`
}

// ─── POST /api/v1/tasks ─────────────────────────────────────────────────────

func CreateTask(c *gin.Context) {
	var req createTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.CreateTask(ctx, &task_pb.CreateTaskRequest{
		TaskType:    task_pb.TaskType(req.TaskType),
		Title:       req.Title,
		Description: req.Description,
		Location:    req.Location,
		RewardDesc:  req.RewardDesc,
		Contact:     req.Contact,
		Note:        req.Note,
		ExpiredAt:   req.ExpiredAt,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── GET /api/v1/tasks ──────────────────────────────────────────────────────

func ListTasks(c *gin.Context) {
	var req listTasksReq
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

	resp, err := client.TaskClient.ListTasks(ctx, &task_pb.ListTasksRequest{
		Cursor:   req.Cursor,
		PageSize: pageSize,
		TaskType: task_pb.TaskType(req.TaskType),
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── GET /api/v1/tasks/:id ──────────────────────────────────────────────────

func GetTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的任务 ID")
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.GetTask(ctx, &task_pb.GetTaskRequest{TaskId: id})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── PUT /api/v1/tasks/:id ──────────────────────────────────────────────────

func UpdateTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的任务 ID")
		return
	}

	var req updateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.UpdateTask(ctx, &task_pb.UpdateTaskRequest{
		TaskId:      id,
		Title:       req.Title,
		Description: req.Description,
		Location:    req.Location,
		RewardDesc:  req.RewardDesc,
		Contact:     req.Contact,
		Note:        req.Note,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── DELETE /api/v1/tasks/:id ───────────────────────────────────────────────

func DeleteTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的任务 ID")
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.DeleteTask(ctx, &task_pb.DeleteTaskRequest{TaskId: id})
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

// ─── POST /api/v1/tasks/:id/claim ───────────────────────────────────────────

func ClaimTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的任务 ID")
		return
	}

	var req claimTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.ClaimTask(ctx, &task_pb.ClaimTaskRequest{
		TaskId:  id,
		Contact: req.Contact,
		Message: req.Message,
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

// ─── PUT /api/v1/tasks/:id/complete ─────────────────────────────────────────

func CompleteTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的任务 ID")
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.CompleteTask(ctx, &task_pb.CompleteTaskRequest{TaskId: id})
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

// ─── PUT /api/v1/tasks/:id/cancel ───────────────────────────────────────────

func CancelTask(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "无效的任务 ID")
		return
	}

	ctx, ok := authCtx(c)
	if !ok {
		return
	}

	resp, err := client.TaskClient.CancelTask(ctx, &task_pb.CancelTaskRequest{TaskId: id})
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
