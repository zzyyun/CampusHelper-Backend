package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	common_pb "go_projects/praProject1/PB/pb/common_pb"
	task_pb "go_projects/praProject1/PB/pb/task_pb"
	"go_projects/praProject1/cmd/task/model"
	"go_projects/praProject1/cmd/task/repo"
	"go_projects/praProject1/pkg/snowflake"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceName = "task-service"

type TaskServiceServer struct {
	task_pb.UnimplementedTaskServiceServer
}

// ─── 常量 ───────────────────────────────────────────────────────────────────

const defaultExpireHours = 24

// ─── CreateTask ─────────────────────────────────────────────────────────────

func (s *TaskServiceServer) CreateTask(ctx context.Context, req *task_pb.CreateTaskRequest) (*task_pb.CreateTaskResponse, error) {
	if req.SchoolId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "school_id/user_id 必须为正数")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, status.Error(codes.InvalidArgument, "标题必填")
	}

	expiredAt := time.Now().Add(defaultExpireHours * time.Hour)
	if req.ExpiredAt > 0 {
		expiredAt = time.Unix(req.ExpiredAt, 0)
	}

	task := &model.Task{
		ID:          snowflake.GenerateID(),
		SchoolID:    req.SchoolId,
		UserID:      req.UserId,
		TaskType:    modelTaskType(req.TaskType),
		Title:       title,
		Description: strings.TrimSpace(req.Description),
		Location:    strings.TrimSpace(req.Location),
		RewardDesc:  strings.TrimSpace(req.RewardDesc),
		Contact:     strings.TrimSpace(req.Contact),
		Note:        strings.TrimSpace(req.Note),
		Status:      model.TaskStatusOpen,
		ExpiredAt:   expiredAt,
	}

	if err := repo.Create(task); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	return &task_pb.CreateTaskResponse{
		TaskId:    task.ID,
		Status:    int64(task.Status),
		CreatedAt: task.CreatedAt.Unix(),
	}, nil
}

// ─── GetTask ────────────────────────────────────────────────────────────────

func (s *TaskServiceServer) GetTask(ctx context.Context, req *task_pb.GetTaskRequest) (*task_pb.Task, error) {
	if req.SchoolId <= 0 || req.TaskId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}

	t, err := repo.GetByID(req.SchoolId, req.TaskId)
	if err != nil {
		if err == repo.ErrNotFound {
			return nil, status.Error(codes.NotFound, "任务不存在")
		}
		return nil, err
	}

	return toPbTask(t, req.UserId), nil
}

// ─── ListTasks ──────────────────────────────────────────────────────────────

func (s *TaskServiceServer) ListTasks(ctx context.Context, req *task_pb.ListTasksRequest) (*task_pb.ListTasksResponse, error) {
	if req.SchoolId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "school_id 必填")
	}

	statusFilter := ""
	if req.Status != task_pb.TaskStatus_TASK_STATUS_UNSPECIFIED {
		statusFilter = fmt.Sprintf("%d", req.Status)
	}

	typeFilter := ""
	if req.TaskType != task_pb.TaskType_TASK_TYPE_UNSPECIFIED {
		typeFilter = req.TaskType.String()
	}

	tasks, hasMore, nextCursor, err := repo.List(req.SchoolId, req.Cursor, int(req.PageSize), typeFilter, statusFilter)
	if err != nil {
		return nil, err
	}

	pbTasks := make([]*task_pb.Task, 0, len(tasks))
	for i := range tasks {
		// 列表中始终不返回联系方式
		pb := toPbTask(&tasks[i], 0)
		pb.Contact = ""
		pb.Note = ""
		pb.ClaimantContact = ""
		pb.ClaimantMsg = ""
		pbTasks = append(pbTasks, pb)
	}

	return &task_pb.ListTasksResponse{
		Tasks:      pbTasks,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// ─── UpdateTask ─────────────────────────────────────────────────────────────

func (s *TaskServiceServer) UpdateTask(ctx context.Context, req *task_pb.UpdateTaskRequest) (*task_pb.Task, error) {
	if req.SchoolId <= 0 || req.TaskId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}

	t, err := repo.GetByID(req.SchoolId, req.TaskId)
	if err != nil {
		return nil, err
	}
	if t.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "仅发布者可编辑")
	}
	if t.Status != model.TaskStatusOpen {
		return nil, status.Error(codes.FailedPrecondition, "仅待接单任务可编辑")
	}

	updates := make(map[string]interface{})
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.RewardDesc != "" {
		updates["reward_desc"] = req.RewardDesc
	}
	if req.Contact != "" {
		updates["contact"] = req.Contact
	}
	if req.Note != "" {
		updates["note"] = req.Note
	}
	if len(updates) == 0 {
		return nil, status.Error(codes.InvalidArgument, "无可更新的字段")
	}

	if err := repo.Update(req.SchoolId, req.UserId, req.TaskId, updates); err != nil {
		return nil, err
	}

	updated, _ := repo.GetByID(req.SchoolId, req.TaskId)
	return toPbTask(updated, req.UserId), nil
}

// ─── DeleteTask ─────────────────────────────────────────────────────────────

func (s *TaskServiceServer) DeleteTask(ctx context.Context, req *task_pb.DeleteTaskRequest) (*common_pb.BaseResponse, error) {
	if req.SchoolId <= 0 || req.TaskId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}

	t, err := repo.GetByID(req.SchoolId, req.TaskId)
	if err != nil {
		return nil, err
	}
	if t.UserID != req.UserId {
		return nil, status.Error(codes.PermissionDenied, "仅发布者可删除")
	}
	if t.Status != model.TaskStatusOpen {
		return nil, status.Error(codes.FailedPrecondition, "仅待接单任务可删除")
	}

	if err := repo.SoftDelete(req.SchoolId, req.UserId, req.TaskId); err != nil {
		return nil, err
	}
	return &common_pb.BaseResponse{Code: 0, Message: "已删除"}, nil
}

// ─── ClaimTask ──────────────────────────────────────────────────────────────

func (s *TaskServiceServer) ClaimTask(ctx context.Context, req *task_pb.ClaimTaskRequest) (*common_pb.BaseResponse, error) {
	if req.SchoolId <= 0 || req.TaskId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}
	if strings.TrimSpace(req.Contact) == "" {
		return nil, status.Error(codes.InvalidArgument, "联系方式必填")
	}

	_, err := repo.Claim(req.SchoolId, req.TaskId, req.UserId, strings.TrimSpace(req.Contact), strings.TrimSpace(req.Message))
	if err != nil {
		switch err {
		case repo.ErrNotFound:
			return nil, status.Error(codes.NotFound, "任务不存在")
		case repo.ErrForbidden:
			return nil, status.Error(codes.PermissionDenied, "不能接自己的任务")
		case repo.ErrInvalidStatus:
			return nil, status.Error(codes.FailedPrecondition, "任务已被接单或状态不允许")
		default:
			return nil, err
		}
	}

	// 返回发布者的联系方式给接单者
	t, err := repo.GetByID(req.SchoolId, req.TaskId)
	if err != nil {
		return &common_pb.BaseResponse{Code: 0, Message: "接单成功"}, nil
	}

	return &common_pb.BaseResponse{
		Code:    0,
		Message: fmt.Sprintf("接单成功。发布者联系方式: %s，留言: %s", t.Contact, t.Note),
	}, nil
}

// ─── CompleteTask ───────────────────────────────────────────────────────────

func (s *TaskServiceServer) CompleteTask(ctx context.Context, req *task_pb.CompleteTaskRequest) (*common_pb.BaseResponse, error) {
	if req.SchoolId <= 0 || req.TaskId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}
	if err := repo.Complete(req.SchoolId, req.TaskId, req.UserId); err != nil {
		if err == repo.ErrNotFound {
			return nil, status.Error(codes.NotFound, "任务不存在或无权操作")
		}
		return nil, err
	}
	return &common_pb.BaseResponse{Code: 0, Message: "已完成"}, nil
}

// ─── CancelTask ─────────────────────────────────────────────────────────────

func (s *TaskServiceServer) CancelTask(ctx context.Context, req *task_pb.CancelTaskRequest) (*common_pb.BaseResponse, error) {
	if req.SchoolId <= 0 || req.TaskId <= 0 || req.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "参数不合法")
	}
	if err := repo.Cancel(req.SchoolId, req.TaskId, req.UserId); err != nil {
		switch err {
		case repo.ErrNotFound:
			return nil, status.Error(codes.NotFound, "任务不存在")
		case repo.ErrForbidden:
			return nil, status.Error(codes.PermissionDenied, "无权取消此任务")
		case repo.ErrInvalidStatus:
			return nil, status.Error(codes.FailedPrecondition, "当前状态不允许取消")
		default:
			return nil, err
		}
	}
	return &common_pb.BaseResponse{Code: 0, Message: "已取消"}, nil
}

// ─── 辅助函数 ───────────────────────────────────────────────────────────────

// toPbTask 将 DB Task 转换为 proto Task。
// viewerID=0 或非相关方时，隐藏联系方式。
func toPbTask(t *model.Task, viewerID int64) *task_pb.Task {
	pb := &task_pb.Task{
		Id:          t.ID,
		SchoolId:    t.SchoolID,
		UserId:      t.UserID,
		ClaimantId:  t.ClaimantID,
		TaskType:    taskTypeToProto(t.TaskType),
		Title:       t.Title,
		Description: t.Description,
		Location:    t.Location,
		RewardDesc:  t.RewardDesc,
		Status:      taskStatusToProto(t.Status),
		ExpiredAt:   t.ExpiredAt.Unix(),
		CreatedAt:   t.CreatedAt.Unix(),
		UpdatedAt:   t.UpdatedAt.Unix(),
	}

	// 联系方式可见性：发布者或接单者可看到对方的联系方式
	if viewerID > 0 && (viewerID == t.UserID || viewerID == t.ClaimantID) {
		if viewerID == t.UserID {
			pb.ClaimantContact = t.ClaimantContact
			pb.ClaimantMsg = t.ClaimantMsg
		}
		if viewerID == t.ClaimantID {
			pb.Contact = t.Contact
			pb.Note = t.Note
		}
	}

	return pb
}

func taskTypeToProto(t string) task_pb.TaskType {
	switch t {
	case "delivery":
		return task_pb.TaskType_TASK_TYPE_DELIVERY
	case "carpool":
		return task_pb.TaskType_TASK_TYPE_CARPOOL
	case "bounty":
		return task_pb.TaskType_TASK_TYPE_BOUNTY
	default:
		return task_pb.TaskType_TASK_TYPE_UNSPECIFIED
	}
}

func taskStatusToProto(s model.TaskStatus) task_pb.TaskStatus {
	switch s {
	case model.TaskStatusOpen:
		return task_pb.TaskStatus_TASK_STATUS_OPEN
	case model.TaskStatusInProgress:
		return task_pb.TaskStatus_TASK_STATUS_IN_PROGRESS
	case model.TaskStatusCompleted:
		return task_pb.TaskStatus_TASK_STATUS_COMPLETED
	case model.TaskStatusCancelled:
		return task_pb.TaskStatus_TASK_STATUS_CANCELLED
	case model.TaskStatusExpired:
		return task_pb.TaskStatus_TASK_STATUS_EXPIRED
	default:
		return task_pb.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

// modelTaskType 将 proto TaskType 枚举转为 model 字符串。
func modelTaskType(t task_pb.TaskType) string {
	switch t {
	case task_pb.TaskType_TASK_TYPE_DELIVERY:
		return "delivery"
	case task_pb.TaskType_TASK_TYPE_CARPOOL:
		return "carpool"
	case task_pb.TaskType_TASK_TYPE_BOUNTY:
		return "bounty"
	default:
		return "delivery"
	}
}

// extractTraceFromMeta 提取 W3C TraceContext。
func extractTraceFromMeta(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	carrier := make(map[string]string)
	for k, vals := range md {
		if len(vals) > 0 {
			carrier[k] = vals[0]
		}
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagationMapCarrier(carrier))
}

type propagationMapCarrier map[string]string

func (c propagationMapCarrier) Get(key string) string  { return c[key] }
func (c propagationMapCarrier) Set(key, value string)  { c[key] = value }
func (c propagationMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// userIDFromCtx reads user-id from gRPC metadata.
func userIDFromCtx(ctx context.Context) int64 {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0
	}
	vals := md.Get("user-id")
	if len(vals) == 0 {
		return 0
	}
	id := int64(0)
	if _, err := fmt.Sscanf(vals[0], "%d", &id); err != nil {
		return 0
	}
	return id
}

// 确保接口兼容性
var _ task_pb.TaskServiceServer = (*TaskServiceServer)(nil)
