package service

import (
	"context"
	"errors"
	"strings"
	"time"

	content_pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/cmd/content/model"
	"go_projects/praProject1/cmd/content/repo"
	"go_projects/praProject1/pkg/snowflake"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 业务校验错误。
var (
	ErrInvalidArgument    = errors.New("参数无效")
	ErrPermissionDenied   = errors.New("无权限操作")
	ErrPostNotFoundBiz    = errors.New("帖子不存在或不属于该学校")
	ErrInvalidStatusBiz   = errors.New("非法的状态流转")
	ErrTitleEmpty         = errors.New("标题不能为空")
	ErrContentEmpty       = errors.New("正文不能为空")
	ErrTitleTooLong       = errors.New("标题不能超过 200 字")
	ErrContentTooLong     = errors.New("正文不能超过 5000 字")
	ErrInvalidPostTypeBiz = errors.New("非法的帖子类型")
)

// PostService 帖子业务服务，承接 gRPC 接口调用。
type PostService struct {
	content_pb.UnimplementedContentServiceServer
	postRepo *repo.PostRepo
	idGen    *snowflake.Snowflake
}

// NewPostService 构造 PostService。
func NewPostService(postRepo *repo.PostRepo, idGen *snowflake.Snowflake) *PostService {
	return &PostService{
		postRepo: postRepo,
		idGen:    idGen,
	}
}

// ─── 帖子 CRUD ─────────────────────────────────────────────────────────────────

// CreatePost 创建帖子。
// 流程：参数校验 → 生成雪花 ID → 写入 DB（status=Pending，等待审核）。
func (s *PostService) CreatePost(ctx context.Context, req *content_pb.CreatePostRequest) (*content_pb.CreatePostResponse, error) {
	// 1. 字段校验
	if err := validatePostType(req.Type); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	title := strings.TrimSpace(req.GetTitle())
	if title == "" {
		return nil, status.Error(codes.InvalidArgument, ErrTitleEmpty.Error())
	}
	if len(title) > 200 {
		return nil, status.Error(codes.InvalidArgument, ErrTitleTooLong.Error())
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, ErrContentEmpty.Error())
	}
	if len(content) > 5000 {
		return nil, status.Error(codes.InvalidArgument, ErrContentTooLong.Error())
	}

	// 2. 生成雪花 ID
	postID, err := s.idGen.NextID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "生成帖子 ID 失败: %v", err)
	}

	// 3. 构造模型
	now := time.Now()
	post := &model.Post{
		ID:        postID,
		SchoolID:  req.GetSchoolId(),
		UserID:    req.GetUserId(),
		Type:      model.PostType(req.GetType()),
		Title:     title,
		Content:   content,
		Images:    model.StringArray(req.GetImages()),
		Status:    model.PostStatusPending, // 默认审核中
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 3. 写入 DB
	if err := s.postRepo.CreatePost(ctx, post); err != nil {
		return nil, status.Errorf(codes.Internal, "创建帖子失败: %v", err)
	}

	return &content_pb.CreatePostResponse{
		PostId:    post.ID,
		Status:    content_pb.PostStatus(post.Status),
		CreatedAt: post.CreatedAt.Unix(),
	}, nil
}

// GetPost 获取帖子详情。
func (s *PostService) GetPost(ctx context.Context, req *content_pb.GetPostRequest) (*content_pb.GetPostResponse, error) {
	if req.GetSchoolId() <= 0 || req.GetPostId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "school_id 或 post_id 无效")
	}
	post, err := s.postRepo.GetPost(ctx, req.GetSchoolId(), req.GetPostId())
	if err != nil {
		if errors.Is(err, repo.ErrPostNotFound) {
			return nil, status.Error(codes.NotFound, "帖子不存在")
		}
		return nil, status.Errorf(codes.Internal, "查询帖子失败: %v", err)
	}
	return &content_pb.GetPostResponse{
		Post:     modelToProto(post),
		IsLiked:  false, // TODO: 点赞功能（Issue #8）实现后接入
		IsOwner:  post.UserID == req.GetViewerUserId(),
	}, nil
}

// UpdatePost 更新帖子。
// 权限校验：仅作者本人可操作。
func (s *PostService) UpdatePost(ctx context.Context, req *content_pb.UpdatePostRequest) (*content_pb.UpdatePostResponse, error) {
	if req.GetSchoolId() <= 0 || req.GetPostId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "school_id 或 post_id 无效")
	}
	// 读取原帖
	post, err := s.postRepo.GetPost(ctx, req.GetSchoolId(), req.GetPostId())
	if err != nil {
		if errors.Is(err, repo.ErrPostNotFound) {
			return nil, status.Error(codes.NotFound, "帖子不存在")
		}
		return nil, status.Errorf(codes.Internal, "查询帖子失败: %v", err)
	}
	// 权限校验
	if post.UserID != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, "仅作者本人可修改帖子")
	}
	// 状态校验：仅 Pending 状态可修改（已发布的帖子不允许直接编辑）
	if post.Status != model.PostStatusPending {
		return nil, status.Error(codes.FailedPrecondition, "仅审核中状态的帖子可修改")
	}

	// 字段更新（仅当客户端提供时更新）
	if title := strings.TrimSpace(req.GetTitle()); title != "" {
		if len(title) > 200 {
			return nil, status.Error(codes.InvalidArgument, ErrTitleTooLong.Error())
		}
		post.Title = title
	}
	if content := strings.TrimSpace(req.GetContent()); content != "" {
		if len(content) > 5000 {
			return nil, status.Error(codes.InvalidArgument, ErrContentTooLong.Error())
		}
		post.Content = content
	}
	if len(req.GetImages()) > 0 {
		post.Images = model.StringArray(req.GetImages())
	}
	post.UpdatedAt = time.Now()

	if err := s.postRepo.UpdatePost(ctx, req.GetSchoolId(), post); err != nil {
		if errors.Is(err, repo.ErrPostNotFound) {
			return nil, status.Error(codes.NotFound, "帖子不存在")
		}
		return nil, status.Errorf(codes.Internal, "更新帖子失败: %v", err)
	}
	return &content_pb.UpdatePostResponse{Success: true}, nil
}

// DeletePost 删除帖子（软删除）。
// 权限校验：仅作者本人可操作。
func (s *PostService) DeletePost(ctx context.Context, req *content_pb.DeletePostRequest) (*content_pb.DeletePostResponse, error) {
	if req.GetSchoolId() <= 0 || req.GetPostId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "school_id 或 post_id 无效")
	}
	// 先查询校验权限
	post, err := s.postRepo.GetPost(ctx, req.GetSchoolId(), req.GetPostId())
	if err != nil {
		if errors.Is(err, repo.ErrPostNotFound) {
			return nil, status.Error(codes.NotFound, "帖子不存在")
		}
		return nil, status.Errorf(codes.Internal, "查询帖子失败: %v", err)
	}
	if post.UserID != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, "仅作者本人可删除帖子")
	}
	if err := s.postRepo.DeletePost(ctx, req.GetSchoolId(), req.GetPostId()); err != nil {
		if errors.Is(err, repo.ErrPostNotFound) {
			return nil, status.Error(codes.NotFound, "帖子不存在")
		}
		return nil, status.Errorf(codes.Internal, "删除帖子失败: %v", err)
	}
	return &content_pb.DeletePostResponse{Success: true}, nil
}

// ListPosts 帖子列表（本期仅返回空列表，详细实现见 Issue #6）。
func (s *PostService) ListPosts(ctx context.Context, req *content_pb.ListPostsRequest) (*content_pb.ListPostsResponse, error) {
	// 占位实现：Issue #6（帖子列表 + 游标分页）将提供完整实现
	return &content_pb.ListPostsResponse{
		Posts:      []*content_pb.Post{},
		NextCursor: "",
		HasMore:    false,
	}, nil
}

// ─── 内部工具方法 ──────────────────────────────────────────────────────────────

// validatePostType 校验 PostType 是否合法。
func validatePostType(t content_pb.PostType) error {
	pt := model.PostType(t)
	if !pt.IsValid() {
		return ErrInvalidPostTypeBiz
	}
	return nil
}

// modelToProto 将 Post 模型转换为 Protobuf 消息。
func modelToProto(p *model.Post) *content_pb.Post {
	if p == nil {
		return nil
	}
	out := &content_pb.Post{
		Id:           p.ID,
		SchoolId:     p.SchoolID,
		UserId:       p.UserID,
		Type:         content_pb.PostType(p.Type),
		Title:        p.Title,
		Content:      p.Content,
		Images:       []string(p.Images),
		Status:       content_pb.PostStatus(p.Status),
		LikesCount:   p.LikesCount,
		CommentCount: p.CommentCount,
		CreatedAt:    timestamppb.New(p.CreatedAt).Seconds,
		UpdatedAt:    timestamppb.New(p.UpdatedAt).Seconds,
	}
	if p.ExpiredAt != nil {
		out.ExpiredAt = timestamppb.New(*p.ExpiredAt).Seconds
	}
	return out
}