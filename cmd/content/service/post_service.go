package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	content_db "go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/cmd/content/repo"
	pb "go_projects/praProject1/PB/pb/content_pb"
	common_pb "go_projects/praProject1/PB/pb/common_pb"
	"go_projects/praProject1/pkg/contextx"
)

// ContentServiceServer 实现 gRPC ContentService 接口
// 遵循 docs/content-service-prd.md §5.1
type ContentServiceServer struct {
	pb.UnimplementedContentServiceServer
}

// ─── 帖子 CRUD ──────────────────────────────────────────────────────────────

// CreatePost 创建帖子（含 DFA 敏感词扫描 → 状态机流转）
func (s *ContentServiceServer) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.CreatePostResponse, error) {
	if req.SchoolId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: school_id/user_id 必须为正数", errInvalidArgument)
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: 标题不能为空", errInvalidArgument)
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("%w: 正文不能为空", errInvalidArgument)
	}

	// ── 1. DFA 敏感词扫描 ────────────────────────────────────────────────
	if hits := ScanSensitive(req.Title + "\n" + req.Content); len(hits) > 0 {
		// Phase 1 仅做预演：返回 sensitive error，状态设为 REJECTED
		// Phase 2 接入 RabbitMQ 异步审核
		return nil, &SensitiveWordErrorType{Hits: hits}
	}

	// ── 2. 序列化图片数组 ──────────────────────────────────────────────
	imagesJSON := "[]"
	if len(req.Images) > 0 {
		b, _ := json.Marshal(req.Images)
		imagesJSON = string(b)
	}

	// ── 3. 构造 Post 模型 ──────────────────────────────────────────────
	post := &content_db.Post{
		ID:         nextPostID(),
		SchoolID:   req.SchoolId,
		UserID:     req.UserId,
		Type:       int8(req.Type),
		Title:      req.Title,
		Content:    req.Content,
		ImagesJSON: imagesJSON,
		Status:     content_db.PostStatusPending, // 默认进入审核中
	}

	// ── 4. 填充业务扩展字段 ────────────────────────────────────────────
	if req.LostFound != nil {
		post.LFType = int8(req.LostFound.GetLostOrFound())
		post.LFLocation = req.LostFound.GetLocation()
		post.LFContact = req.LostFound.GetContact()
		post.LFCategory = int8(req.LostFound.GetItemCategory())
	}
	if req.SecondHand != nil {
		post.SHPrice = req.SecondHand.GetPrice()
		post.SHOriginal = req.SecondHand.GetOriginalPrice()
		post.SHCondition = int8(req.SecondHand.GetCondition())
		post.SHTradeMethod = int8(req.SecondHand.GetTradeMethod())
		post.SHCategory = int8(req.SecondHand.GetItemCategory())
	}

	// ── 5. 写入数据库 ──────────────────────────────────────────────────
	if err := repo.Create(post); err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}

	// 注入 trace_id 便于链路追踪
	contextx.SetTraceID(ctx, traceIDFromContext(ctx))

	return &pb.CreatePostResponse{
		PostId:    post.ID,
		Status:    pb.PostStatus(post.Status),
		CreatedAt: post.CreatedAt.Unix(),
	}, nil
}

// GetPost 获取帖子详情
func (s *ContentServiceServer) GetPost(ctx context.Context, req *pb.GetPostRequest) (*pb.GetPostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: school_id/post_id 必须为正数", errInvalidArgument)
	}

	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("%w: post %d 不存在", errNotFound, req.PostId)
		}
		return nil, err
	}

	resp := &pb.GetPostResponse{
		Post:     toPbPost(post),
		IsOwner:  post.UserID == req.ViewerUserId,
		IsLiked:  false,
	}

	// 查询点赞状态
	if req.ViewerUserId > 0 {
		liked, err := repo.HasLiked(req.SchoolId, req.PostId, req.ViewerUserId)
		if err == nil {
			resp.IsLiked = liked
		}
	}

	return resp, nil
}

// UpdatePost 更新帖子（仅作者本人）
func (s *ContentServiceServer) UpdatePost(ctx context.Context, req *pb.UpdatePostRequest) (*pb.UpdatePostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}

	fields := map[string]interface{}{
		"title":   req.Title,
		"content": req.Content,
	}
	if len(req.Images) > 0 {
		b, _ := json.Marshal(req.Images)
		fields["images"] = string(b)
	}
	if req.LostFound != nil {
		fields["lf_type"] = int8(req.LostFound.GetLostOrFound())
		fields["lf_location"] = req.LostFound.GetLocation()
		fields["lf_contact"] = req.LostFound.GetContact()
		fields["lf_category"] = int8(req.LostFound.GetItemCategory())
	}
	if req.SecondHand != nil {
		fields["sh_price"] = req.SecondHand.GetPrice()
		fields["sh_original_price"] = req.SecondHand.GetOriginalPrice()
		fields["sh_condition"] = int8(req.SecondHand.GetCondition())
		fields["sh_trade_method"] = int8(req.SecondHand.GetTradeMethod())
		fields["sh_category"] = int8(req.SecondHand.GetItemCategory())
	}

	if err := repo.UpdateOwned(req.SchoolId, req.UserId, req.PostId, fields); err != nil {
		if errors.Is(err, repo.ErrForbidden) {
			return nil, fmt.Errorf("%w: 非作者本人或记录不存在", errForbidden)
		}
		return nil, err
	}
	return &pb.UpdatePostResponse{Success: true}, nil
}

// DeletePost 删除帖子（软删除，仅作者本人）
func (s *ContentServiceServer) DeletePost(ctx context.Context, req *pb.DeletePostRequest) (*pb.DeletePostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}
	if err := repo.DeleteOwned(req.SchoolId, req.UserId, req.PostId); err != nil {
		if errors.Is(err, repo.ErrForbidden) {
			return nil, fmt.Errorf("%w: 非作者本人或记录不存在", errForbidden)
		}
		return nil, err
	}
	return &pb.DeletePostResponse{Success: true}, nil
}

// ListPosts 帖子列表（游标分页）
func (s *ContentServiceServer) ListPosts(ctx context.Context, req *pb.ListPostsRequest) (*pb.ListPostsResponse, error) {
	if req.SchoolId <= 0 {
		return nil, fmt.Errorf("%w: school_id 必须为正数", errInvalidArgument)
	}

	pageSize := 20
	var cursor int64
	if req.Pagination != nil {
		pageSize = int(req.Pagination.GetPageSize())
		if pageSize <= 0 {
			pageSize = 20
		}
		cursor = parseCursor(req.Pagination.GetCursor())
	}

	status := int8(pb.PostStatus_POST_STATUS_PUBLISHED) // 默认只查已发布
	if req.Status != pb.PostStatus_POST_STATUS_UNSPECIFIED {
		status = int8(req.Status)
	}

	posts, nextCursor, err := repo.ListByCursor(req.SchoolId, int8(req.Type), status, cursor, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListPostsResponse{
		Posts:      make([]*pb.Post, 0, len(posts)),
		HasMore:    nextCursor > 0,
		NextCursor: fmt.Sprintf("%d", nextCursor),
	}
	for i := range posts {
		resp.Posts = append(resp.Posts, toPbPost(&posts[i]))
	}
	return resp, nil
}

// ─── 点赞 ───────────────────────────────────────────────────────────────────

// LikePost 点赞帖子
func (s *ContentServiceServer) LikePost(ctx context.Context, req *pb.LikePostRequest) (*pb.LikePostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}

	added, err := repo.AddLike(req.SchoolId, req.PostId, req.UserId)
	if err != nil {
		return nil, err
	}
	if added {
		repo.IncLikesCount(req.PostId)
	}

	// 查询最新点赞数
	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		return nil, err
	}
	return &pb.LikePostResponse{Liked: true, LikesCount: post.LikesCount}, nil
}

// UnlikePost 取消点赞
func (s *ContentServiceServer) UnlikePost(ctx context.Context, req *pb.UnlikePostRequest) (*pb.UnlikePostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}

	removed, err := repo.RemoveLike(req.SchoolId, req.PostId, req.UserId)
	if err != nil {
		return nil, err
	}
	if removed {
		repo.DecLikesCount(req.PostId)
	}

	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		return nil, err
	}
	return &pb.UnlikePostResponse{Liked: false, LikesCount: post.LikesCount}, nil
}

// ─── 评论 / 搜索 / 审核（Phase 1 占位） ────────────────────────────────────

// CreateComment 创建评论（Phase 1 占位，后续 Issue 实现）
func (s *ContentServiceServer) CreateComment(ctx context.Context, req *pb.CreateCommentRequest) (*pb.CreateCommentResponse, error) {
	return nil, fmt.Errorf("%w: 评论功能待实现", errUnimplemented)
}

// DeleteComment 删除评论（Phase 1 占位）
func (s *ContentServiceServer) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.DeleteCommentResponse, error) {
	return nil, fmt.Errorf("%w: 评论功能待实现", errUnimplemented)
}

// ListComments 评论列表（Phase 1 占位）
func (s *ContentServiceServer) ListComments(ctx context.Context, req *pb.ListCommentsRequest) (*pb.ListCommentsResponse, error) {
	return nil, fmt.Errorf("%w: 评论功能待实现", errUnimplemented)
}

// SearchContent 搜索内容（Phase 1 占位，ES 接入待实现）
func (s *ContentServiceServer) SearchContent(ctx context.Context, req *pb.SearchContentRequest) (*pb.SearchContentResponse, error) {
	return nil, fmt.Errorf("%w: 搜索功能待实现", errUnimplemented)
}

// ApprovePost 审核通过（pending → published，Phase 1 占位）
func (s *ContentServiceServer) ApprovePost(ctx context.Context, req *pb.ApprovePostRequest) (*pb.ApprovePostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}
	if err := repo.UpdateStatus(req.SchoolId, req.PostId,
		content_db.PostStatusPending, content_db.PostStatusPublished); err != nil {
		return nil, fmt.Errorf("approve: %w", err)
	}
	return &pb.ApprovePostResponse{Success: true, NewStatus: pb.PostStatus_POST_STATUS_PUBLISHED}, nil
}

// RejectPost 审核拒绝（pending → rejected，Phase 1 占位）
func (s *ContentServiceServer) RejectPost(ctx context.Context, req *pb.RejectPostRequest) (*pb.RejectPostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("%w: 拒绝原因必填", errInvalidArgument)
	}
	if err := repo.UpdateStatus(req.SchoolId, req.PostId,
		content_db.PostStatusPending, content_db.PostStatusRejected); err != nil {
		return nil, fmt.Errorf("reject: %w", err)
	}
	return &pb.RejectPostResponse{Success: true, NewStatus: pb.PostStatus_POST_STATUS_REJECTED}, nil
}

// TakedownPost 违规下架（published → closed，Phase 1 占位）
func (s *ContentServiceServer) TakedownPost(ctx context.Context, req *pb.TakedownPostRequest) (*pb.TakedownPostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}
	if err := repo.UpdateStatus(req.SchoolId, req.PostId,
		content_db.PostStatusPublished, content_db.PostStatusClosed); err != nil {
		return nil, fmt.Errorf("takedown: %w", err)
	}
	return &pb.TakedownPostResponse{Success: true, NewStatus: pb.PostStatus_POST_STATUS_CLOSED}, nil
}

// ─── 辅助函数 ───────────────────────────────────────────────────────────────

// toPbPost 把 DB 模型转换为 protobuf 消息
func toPbPost(p *content_db.Post) *pb.Post {
	if p == nil {
		return nil
	}
	var images []string
	if p.ImagesJSON != "" {
		_ = json.Unmarshal([]byte(p.ImagesJSON), &images)
	}
	out := &pb.Post{
		Id:           p.ID,
		SchoolId:     p.SchoolID,
		UserId:       p.UserID,
		Type:         pb.PostType(p.Type),
		Title:        p.Title,
		Content:      p.Content,
		Images:       images,
		Status:       pb.PostStatus(p.Status),
		LikesCount:   p.LikesCount,
		CommentCount: p.CommentCnt,
		CreatedAt:    p.CreatedAt.Unix(),
		UpdatedAt:    p.UpdatedAt.Unix(),
	}
	if p.ExpiredAt != nil {
		out.ExpiredAt = p.ExpiredAt.Unix()
	}
	// 业务扩展字段
	if pb.PostType(p.Type) == pb.PostType_POST_TYPE_LOST_FOUND {
		out.LostFound = &pb.LostFoundExtra{
			Location:     p.LFLocation,
			ItemCategory: pb.ItemCategory(p.LFCategory),
			Contact:      p.LFContact,
			LostOrFound:  pb.LostOrFoundType(p.LFType),
		}
	}
	if pb.PostType(p.Type) == pb.PostType_POST_TYPE_SECOND_HAND {
		out.SecondHand = &pb.SecondHandExtra{
			Price:         p.SHPrice,
			OriginalPrice: p.SHOriginal,
			Condition:     pb.Condition(p.SHCondition),
			TradeMethod:   pb.TradeMethod(p.SHTradeMethod),
			ItemCategory:  pb.ItemCategory(p.SHCategory),
		}
	}
	return out
}

// parseCursor 解析游标字符串（目前是十进制 ID）
func parseCursor(s string) int64 {
	if s == "" {
		return 0
	}
	var id int64
	_, _ = fmt.Sscanf(s, "%d", &id)
	return id
}

// traceIDFromContext 提取 trace_id（trace 透传在 pkg/middleware 层统一处理）
func traceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextx.TraceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// ─── 错误常量（Phase 1 简化处理，Phase 2 接入 pkg/errcode） ────────────────

var (
	errInvalidArgument = errors.New("invalid argument")
	errNotFound        = errors.New("not found")
	errForbidden       = errors.New("permission denied")
	errUnimplemented   = errors.New("unimplemented")
)

// SensitiveWordErrorType 敏感词错误类型（用于 errors.As 提取）
// gRPC gateway 拦截后可转换为 SensitiveWordError 响应
type SensitiveWordErrorType struct {
	Hits []*pb.SensitiveWordHit
}

func (e *SensitiveWordErrorType) Error() string {
	return "内容包含敏感词"
}

// AsSensitiveWordError 从 err 提取 SensitiveWordErrorType
func AsSensitiveWordError(err error) (*SensitiveWordErrorType, bool) {
	var s *SensitiveWordErrorType
	if errors.As(err, &s) {
		return s, true
	}
	return nil, false
}

// 确保 common_pb 引用被使用（后续排序扩展会用）
var _ = common_pb.SortType_SORT_TYPE_UNSPECIFIED