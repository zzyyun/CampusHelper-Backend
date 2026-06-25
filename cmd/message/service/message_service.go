package service

import (
	"context"
	"fmt"
	"log"
	"time"

	message_pb "go_projects/praProject1/PB/pb/message_pb"
	"go_projects/praProject1/cmd/message/repo"

	common_pb "go_projects/praProject1/PB/pb/common_pb"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceName = "message-service"

// MessageServiceServer 实现 gRPC MessageService 接口。
type MessageServiceServer struct {
	message_pb.UnimplementedMessageServiceServer
}

// ─── ListNotifications ──────────────────────────────────────────────────────

func (s *MessageServiceServer) ListNotifications(ctx context.Context, req *message_pb.ListNotificationsRequest) (*message_pb.ListNotificationsResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "MessageService.ListNotifications")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "缺少用户身份")
	}
	schoolID := req.GetSchoolId()

	span.SetAttributes(
		attribute.Int64("user.id", userID),
		attribute.Int64("school.id", schoolID),
	)

	notifications, hasMore, nextCursor, err := repo.ListByUser(userID, schoolID,
		req.GetCursor(), int(req.GetPageSize()), req.GetType())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("查询通知列表: %w", err)
	}

	unreadCount, err := repo.UnreadCount(userID, schoolID)
	if err != nil {
		span.RecordError(err)
		// 未读数查询失败不阻塞列表返回，记录日志即可
		log.Printf("[message-service] 查询未读数失败: %v", err)
	}

	pbNotifications := make([]*message_pb.Notification, 0, len(notifications))
	for i := range notifications {
		n := &notifications[i]
		pbNotifications = append(pbNotifications, &message_pb.Notification{
			Id:         n.ID,
			Type:       n.Type,
			Title:      n.Title,
			Content:    n.Content,
			FromUserId: n.FromUserID,
			RefType:    n.RefType,
			RefId:      n.RefID,
			IsRead:     n.IsRead,
			CreatedAt:  n.CreatedAt.Unix(),
		})
	}

	return &message_pb.ListNotificationsResponse{
		Notifications: pbNotifications,
		UnreadCount:   unreadCount,
		HasMore:       hasMore,
		NextCursor:    nextCursor,
	}, nil
}

// ─── UnreadCount ────────────────────────────────────────────────────────────

func (s *MessageServiceServer) UnreadCount(ctx context.Context, req *message_pb.UnreadCountRequest) (*message_pb.UnreadCountResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "MessageService.UnreadCount")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "缺少用户身份")
	}

	count, err := repo.UnreadCount(userID, req.GetSchoolId())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("查询未读数: %w", err)
	}

	return &message_pb.UnreadCountResponse{Count: count}, nil
}

// ─── MarkRead ───────────────────────────────────────────────────────────────

func (s *MessageServiceServer) MarkRead(ctx context.Context, req *message_pb.MarkReadRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "MessageService.MarkRead")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "缺少用户身份")
	}

	if req.GetId() <= 0 {
		return &common_pb.BaseResponse{Code: 400, Message: "通知 ID 必须为正数"}, nil
	}

	if err := repo.MarkRead(req.GetId(), userID); err != nil {
		span.RecordError(err)
		return &common_pb.BaseResponse{Code: 404, Message: "通知不存在或无权操作"}, nil
	}

	return &common_pb.BaseResponse{Code: 0, Message: "ok"}, nil
}

// ─── MarkAllRead ────────────────────────────────────────────────────────────

func (s *MessageServiceServer) MarkAllRead(ctx context.Context, req *message_pb.MarkAllReadRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "MessageService.MarkAllRead")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "缺少用户身份")
	}

	affected, err := repo.MarkAllRead(userID, req.GetSchoolId())
	if err != nil {
		span.RecordError(err)
		return &common_pb.BaseResponse{Code: 500, Message: "标记全部已读失败"}, nil
	}

	return &common_pb.BaseResponse{Code: 0, Message: fmt.Sprintf("已将 %d 条通知标记为已读", affected)}, nil
}

// ─── DeleteNotification ─────────────────────────────────────────────────────

func (s *MessageServiceServer) DeleteNotification(ctx context.Context, req *message_pb.DeleteNotificationRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "MessageService.DeleteNotification")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		userID = req.GetUserId()
	}
	if userID == 0 {
		return nil, status.Error(codes.Unauthenticated, "缺少用户身份")
	}

	if req.GetId() <= 0 {
		return &common_pb.BaseResponse{Code: 400, Message: "通知 ID 必须为正数"}, nil
	}

	if err := repo.SoftDelete(req.GetId(), userID); err != nil {
		span.RecordError(err)
		return &common_pb.BaseResponse{Code: 404, Message: "通知不存在或无权操作"}, nil
	}

	return &common_pb.BaseResponse{Code: 0, Message: "已删除"}, nil
}

// ─── 辅助函数 ───────────────────────────────────────────────────────────────

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

// 确保接口兼容
var _ message_pb.MessageServiceServer = (*MessageServiceServer)(nil)

// 抑制未使用导入
var _ = time.Now