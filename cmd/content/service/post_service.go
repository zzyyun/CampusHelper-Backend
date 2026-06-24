package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	content_db "go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/cmd/content/repo"
	pb "go_projects/praProject1/PB/pb/content_pb"
	common_pb "go_projects/praProject1/PB/pb/common_pb"
	"go_projects/praProject1/pkg/contextx"
	es_pkg "go_projects/praProject1/pkg/es"
	"go_projects/praProject1/pkg/mq"
)

// ContentServiceServer 实现 gRPC ContentService 接口
// 遵循 docs/content-service-prd.md §5.1
type ContentServiceServer struct {
	pb.UnimplementedContentServiceServer
}

// mqPublisher 全局消息发布者，由 main.go 中的 InitMQ 初始化。
// 若未初始化（nil），审核操作仅记录日志不发布消息。
var mqPublisher *mq.Publisher

// InitMQ 初始化 RabbitMQ 发布者。
// addr 格式: amqp://user:pass@host:port/
func InitMQ(addr string) {
	mqPublisher = mq.NewPublisher(addr, "content.events")
	log.Printf("[content-service] MQ Publisher 已初始化（队列: content.events）")
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
	// Phase 1 实现：先落库（pending），再由异步审核任务判定。
	// 当前同步路径检测到敏感词时直接拒绝（不落库），避免脏数据堆积。
	// Phase 2 接入 RabbitMQ 后改为：先落库 pending → 异步扫描 → 命中则 UpdateStatus(rejected) + 写 reason。
	if hits := ScanSensitive(req.Title + "\n" + req.Content); len(hits) > 0 {
		return nil, &SensitiveWordErrorType{Hits: hits}
	}

	// ── 2. 序列化图片数组 ──────────────────────────────────────────────
	imagesJSON := "[]"
	if len(req.Images) > 0 {
		b, _ := json.Marshal(req.Images)
		imagesJSON = string(b)
	}

	// ── 3. 构造 Post 模型 ──────────────────────────────────────────────
	postID, err := nextPostID()
	if err != nil {
		// 雪花 ID 生成失败（时钟回拨等）必须 fail-fast，禁止写入 ID=0 的脏数据
		return nil, fmt.Errorf("%w: 生成帖子 ID 失败: %v", errInvalidArgument, err)
	}
	post := &content_db.Post{
		ID:         postID,
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

	// 链路追踪由 pkg/middleware/tracing 拦截器统一注入 ctx，
	// 此处不再重复调用 contextx.SetTraceID（否则结果 ctx 被丢弃，属于死代码）。
	// 如需在响应中回带 trace_id，请在 gRPC 响应 Header 中由统一拦截器写入。

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
		NextCursor: encodeCursor(nextCursor),
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

	// 查询最新点赞数 + 真实 like 状态（避免"已点赞"被误报为 false）
	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		return nil, err
	}
	liked, err := repo.HasLiked(req.SchoolId, req.PostId, req.UserId)
	if err != nil {
		return nil, err
	}
	return &pb.LikePostResponse{Liked: liked, LikesCount: post.LikesCount}, nil
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
	liked, err := repo.HasLiked(req.SchoolId, req.PostId, req.UserId)
	if err != nil {
		return nil, err
	}
	return &pb.UnlikePostResponse{Liked: liked, LikesCount: post.LikesCount}, nil
}

// ─── 评论 / 搜索 / 审核（Phase 1 占位） ────────────────────────────────────

// CreateComment 创建一级评论（含 DFA 敏感词扫描）。
//
// 流程：
//  1. 校验参数（school_id/post_id/user_id > 0, content 1-500 字）
//  2. DFA 敏感词扫描
//  3. 生成雪花 ID，写入数据库（事务内原子递增 comment_count）
func (s *ContentServiceServer) CreateComment(ctx context.Context, req *pb.CreateCommentRequest) (*pb.CreateCommentResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: 评论内容不能为空", errInvalidArgument)
	}
	// 按字符数限制（runes），500 字
	runes := []rune(content)
	if len(runes) > 500 {
		return nil, fmt.Errorf("%w: 评论内容不能超过 500 字", errInvalidArgument)
	}

	// DFA 敏感词扫描
	if hits := ScanSensitive(content); len(hits) > 0 {
		return nil, &SensitiveWordErrorType{Hits: hits}
	}

	// 生成评论 ID
	commentID, err := nextCommentID()
	if err != nil {
		return nil, fmt.Errorf("%w: 生成评论 ID 失败: %v", errInvalidArgument, err)
	}

	comment := &content_db.PostComment{
		ID:       commentID,
		SchoolID: req.SchoolId,
		PostID:   req.PostId,
		UserID:   req.UserId,
		Content:  content,
		ParentID: 0,  // Phase 1: 一级评论
		Status:   1,  // 正常
	}

	if err := repo.CreateComment(comment); err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	return &pb.CreateCommentResponse{
		CommentId: comment.ID,
		CreatedAt: comment.CreatedAt.Unix(),
	}, nil
}

// DeleteComment 删除评论（仅作者本人）。
//
// 流程：
//  1. 校验参数
//  2. 事务内：校验 ownership + 软删除 + 原子递减 comment_count
func (s *ContentServiceServer) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.DeleteCommentResponse, error) {
	if req.SchoolId <= 0 || req.CommentId <= 0 || req.UserId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}

	deleted, err := repo.DeleteOwnedComment(req.SchoolId, req.CommentId, req.UserId)
	if err != nil {
		if errors.Is(err, repo.ErrForbidden) {
			return nil, fmt.Errorf("%w: 非作者本人或评论不存在", errForbidden)
		}
		return nil, err
	}

	return &pb.DeleteCommentResponse{Success: deleted}, nil
}

// ListComments 评论列表（游标分页，正序）。
//
// 分页策略：游标为上一页最后一条评论的 ID。
func (s *ContentServiceServer) ListComments(ctx context.Context, req *pb.ListCommentsRequest) (*pb.ListCommentsResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
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

	comments, nextCursor, err := repo.ListComments(req.SchoolId, req.PostId, cursor, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListCommentsResponse{
		Comments: make([]*pb.Comment, 0, len(comments)),
		HasMore:  nextCursor > 0,
	}
	if nextCursor > 0 {
		resp.NextCursor = encodeCursor(nextCursor)
	}
	for i := range comments {
		resp.Comments = append(resp.Comments, toPbComment(&comments[i]))
	}
	return resp, nil
}

// esClient 全局 ES 客户端，由 InitES 初始化。
var esClient *es_pkg.Client

// InitES 初始化 Elasticsearch 客户端。
func InitES(addrs []string) {
	var err error
	esClient, err = es_pkg.NewClient(addrs, "campus_posts")
	if err != nil {
		log.Printf("[content-service] ES 初始化失败（搜索功能降级）: %v", err)
		return
	}
	log.Printf("[content-service] ES 客户端已初始化（索引: campus_posts）")
}

// SearchContent 搜索内容（ES 关键词搜索 + 分类筛选）。
//
// 搜索维度：
//   - keyword：同时匹配 title 和 content 字段
//   - type：帖子分类筛选（可选）
//   - category：物品分类筛选（可选）
//   - status：状态筛选（可选，默认已发布）
//   - page/page_size：分页
//   - sort：排序方式（默认按创建时间倒序）
func (s *ContentServiceServer) SearchContent(ctx context.Context, req *pb.SearchContentRequest) (*pb.SearchContentResponse, error) {
	if req.SchoolId <= 0 {
		return nil, fmt.Errorf("%w: school_id 必须为正数", errInvalidArgument)
	}
	if esClient == nil {
		return nil, fmt.Errorf("搜索服务暂不可用（ES 未连接）")
	}
	if strings.TrimSpace(req.Keyword) == "" {
		return nil, fmt.Errorf("%w: 搜索关键词不能为空", errInvalidArgument)
	}

	// 默认值处理
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	from := (page - 1) * pageSize

	// 构建 ES 搜索查询
	queryJSON := buildSearchQuery(req.SchoolId, req.Keyword, req.Type, req.Category, req.Status, from, pageSize, req.Sort)
	result, err := esClient.Search(ctx, queryJSON)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	// 映射搜索结果
	posts := make([]*pb.Post, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		doc := &content_db.Post{
			ID:           hit.Source.PostID,
			SchoolID:     hit.Source.SchoolID,
			UserID:       hit.Source.UserID,
			Type:         hit.Source.Type,
			Title:        hit.Source.Title,
			Content:      hit.Source.Content,
			Status:       content_db.PostStatus(hit.Source.Status),
			LikesCount:   hit.Source.LikesCount,
			CommentCount: hit.Source.CommentCount,
		}
		posts = append(posts, toPbPost(doc))
	}

	return &pb.SearchContentResponse{
		Posts:    posts,
		Total:    result.Hits.Total.Value,
		Page:     int32(page),
		PageSize: int32(pageSize),
	}, nil
}

// ApprovePost 审核通过（pending → published，发布 MQ 事件触发 ES 同步）。
//
// 流程：
//  1. 校验参数（school_id > 0, post_id > 0）
//  2. 先查询帖子获取 user_id（用于 MQ 事件），再状态机校验（仅 pending 可审核通过）
//  3. 原子更新：status=published + reviewer_id + reviewed_at
//  4. 发送 MQ content.published 事件（best-effort，失败不阻塞审核）
func (s *ContentServiceServer) ApprovePost(ctx context.Context, req *pb.ApprovePostRequest) (*pb.ApprovePostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}

	// 先查询帖子（用于校验状态 + 获取 user_id 构造 MQ 事件）
	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("%w: 帖子不存在", errNotFound)
		}
		return nil, err
	}

	// 状态机校验
	if post.Status != content_db.PostStatusPending {
		return nil, fmt.Errorf("%w: 只有待审核（pending）状态可以审核通过，当前状态 %d", errInvalidArgument, post.Status)
	}

	// 原子更新状态 + 审核信息
	if err := repo.UpdateReview(req.SchoolId, req.PostId,
		content_db.PostStatusPending, content_db.PostStatusPublished,
		req.ReviewerId, ""); err != nil {
		return nil, fmt.Errorf("approve: %w", err)
	}

	// 发送 MQ 事件（best-effort，失败不阻塞审核结果）
	publishEvent(ctx, mq.EventContentPublished, post.ID, post.SchoolID, post.UserID)

	return &pb.ApprovePostResponse{Success: true, NewStatus: pb.PostStatus_POST_STATUS_PUBLISHED}, nil
}

// RejectPost 审核拒绝（pending → rejected，需填拒绝原因，发布 MQ 事件通知用户）。
//
// 流程：
//  1. 校验参数 + 拒绝原因非空
//  2. 状态机校验（仅 pending 可审核拒绝）
//  3. 原子更新：status=rejected + reviewer_id + reviewed_at + reject_reason
//  4. 发送 MQ content.review_result 事件（通知用户审核未通过）
func (s *ContentServiceServer) RejectPost(ctx context.Context, req *pb.RejectPostRequest) (*pb.RejectPostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: 拒绝原因必填", errInvalidArgument)
	}

	// 先查询帖子（校验状态 + 获取 user_id）
	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("%w: 帖子不存在", errNotFound)
		}
		return nil, err
	}

	if post.Status != content_db.PostStatusPending {
		return nil, fmt.Errorf("%w: 只有待审核（pending）状态可以审核拒绝，当前状态 %d", errInvalidArgument, post.Status)
	}

	// 原子更新状态 + 审核信息
	if err := repo.UpdateReview(req.SchoolId, req.PostId,
		content_db.PostStatusPending, content_db.PostStatusRejected,
		req.ReviewerId, reason); err != nil {
		return nil, fmt.Errorf("reject: %w", err)
	}

	// 发送 MQ 事件（通知用户）
	event := mq.NewContentEvent(mq.EventContentRejected, post.ID, post.SchoolID, post.UserID,
		traceIDFromContext(ctx))
	event.Data["result"] = "rejected"
	event.Data["reason"] = reason
	publishEventRaw(event)

	return &pb.RejectPostResponse{Success: true, NewStatus: pb.PostStatus_POST_STATUS_REJECTED}, nil
}

// TakedownPost 违规下架（published → closed，发布 MQ 事件通知 ES 删除文档）。
//
// 流程：
//  1. 校验参数
//  2. 状态机校验（仅 published 可下架）
//  3. 原子更新：status=closed + reviewer_id + reviewed_at + reject_reason
//  4. 发送 MQ content.taken_down 事件
func (s *ContentServiceServer) TakedownPost(ctx context.Context, req *pb.TakedownPostRequest) (*pb.TakedownPostResponse, error) {
	if req.SchoolId <= 0 || req.PostId <= 0 {
		return nil, fmt.Errorf("%w: 参数不合法", errInvalidArgument)
	}

	// 先查询帖子
	post, err := repo.GetByID(req.SchoolId, req.PostId)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("%w: 帖子不存在", errNotFound)
		}
		return nil, err
	}

	if post.Status != content_db.PostStatusPublished {
		return nil, fmt.Errorf("%w: 只有已发布（published）状态可以下架，当前状态 %d", errInvalidArgument, post.Status)
	}

	// 原子更新状态 + 审核信息（下架原因存入 reject_reason）
	if err := repo.UpdateReview(req.SchoolId, req.PostId,
		content_db.PostStatusPublished, content_db.PostStatusClosed,
		req.ReviewerId, req.Reason); err != nil {
		return nil, fmt.Errorf("takedown: %w", err)
	}

	// 发送 MQ 事件
	event := mq.NewContentEvent(mq.EventContentTakenDown, post.ID, post.SchoolID, post.UserID,
		traceIDFromContext(ctx))
	if req.Reason != "" {
		event.Data["reason"] = req.Reason
	}
	publishEventRaw(event)

	return &pb.TakedownPostResponse{Success: true, NewStatus: pb.PostStatus_POST_STATUS_CLOSED}, nil}

// ─── MQ 辅助函数 ─────────────────────────────────────────────────────────────

// publishEvent 发布简单的 MQ 内容事件。
func publishEvent(ctx context.Context, eventType string, postID, schoolID, userID int64) {
	event := mq.NewContentEvent(eventType, postID, schoolID, userID, traceIDFromContext(ctx))
	publishEventRaw(event)
}

// publishEventRaw 发布事件（best-effort，失败仅记录日志）。
func publishEventRaw(event *mq.ContentEvent) {
	if mqPublisher == nil {
		log.Printf("[content-service] MQ 未初始化，跳过事件发布: type=%s post=%d", event.Type, event.PostID)
		return
	}
	// 使用独立的 context（不依赖 gRPC ctx 生命周期）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = mqPublisher.Publish(ctx, event) // Publish 内部已有降级日志
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
		CommentCount: p.CommentCount,
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

// toPbComment 把 DB 评论模型转换为 protobuf 消息
func toPbComment(c *content_db.PostComment) *pb.Comment {
	if c == nil {
		return nil
	}
	return &pb.Comment{
		Id:        c.ID,
		SchoolId:  c.SchoolID,
		PostId:    c.PostID,
		UserId:    c.UserID,
		Content:   c.Content,
		ParentId:  c.ParentID,
		Status:    pb.CommentStatus(c.Status),
		CreatedAt: c.CreatedAt.Unix(),
	}
}

// encodeCursor 将游标编码为 Base64+JSON 字符串。
// 格式：Base64({"last_id":12345})，便于后续扩展字段（如 sort_value）。
func encodeCursor(lastID int64) string {
	if lastID <= 0 {
		return ""
	}
	payload := fmt.Sprintf(`{"last_id":%d}`, lastID)
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

// parseCursor 解析游标字符串。
// 向后兼容旧版纯数字游标，同时支持 Base64+JSON 格式。
func parseCursor(s string) int64 {
	if s == "" {
		return 0
	}
	// 尝试 Base64 解码（新版格式）
	data, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		var p struct {
			LastID int64 `json:"last_id"`
		}
		if json.Unmarshal(data, &p) == nil && p.LastID > 0 {
			return p.LastID
		}
	}
	// 回退：旧版纯数字格式
	var id int64
	_, _ = fmt.Sscanf(s, "%d", &id)
	return id
}

// traceIDFromContext 提取 trace_id（trace 透传在 pkg/middleware 层统一处理）
func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(contextx.TraceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// buildSearchQuery 构建 ES 搜索查询 JSON。
// 查询维度：school_id（必须）+ keyword（必须） + type/category/status（可选）。
func buildSearchQuery(schoolID int64, keyword string, postType pb.PostType, category pb.ItemCategory,
	status pb.PostStatus, from, size int, sort common_pb.SortType) string {

	// 构建 filter 子句（可叠加）
	var filters []string
	filters = append(filters, fmt.Sprintf(`{"term":{"school_id":%d}}`, schoolID))

	if postType != pb.PostType_POST_TYPE_UNSPECIFIED {
		filters = append(filters, fmt.Sprintf(`{"term":{"type":%d}}`, postType))
	}

	if category != pb.ItemCategory_ITEM_CATEGORY_UNSPECIFIED {
		// 分类同时匹配失物招领和二手交易的分类字段
		filters = append(filters,
			fmt.Sprintf(`{"bool":{"should":[{"term":{"lf_category":%d}},{"term":{"sh_category":%d}}]}}`, category, category))
	}

	if status != pb.PostStatus_POST_STATUS_UNSPECIFIED {
		filters = append(filters, fmt.Sprintf(`{"term":{"status":%d}}`, status))
	}

	// 构建排序
	var sortClause string
	switch sort {
	case common_pb.SortType_SORT_TYPE_TIME_DESC, common_pb.SortType_SORT_TYPE_UNSPECIFIED:
		sortClause = `[{"created_at":{"order":"desc"}}]`
	case common_pb.SortType_SORT_TYPE_LIKES_DESC:
		sortClause = `[{"likes_count":{"order":"desc"}}]`
	case common_pb.SortType_SORT_TYPE_RELEVANCE:
		sortClause = `[{"_score":{"order":"desc"}}]`
	default:
		sortClause = `[{"created_at":{"order":"desc"}}]`
	}

	return fmt.Sprintf(
		`{"from":%d,"size":%d,"query":{"bool":{"must":[{"multi_match":{"query":%q,"fields":["title","content"],"type":"best_fields"}}],"filter":[%s]}},"sort":%s}`,
		from, size, keyword, strings.Join(filters, ","), sortClause,
	)
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