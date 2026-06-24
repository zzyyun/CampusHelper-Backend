package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	common_pb "go_projects/praProject1/PB/pb/common_pb"
	content_pb "go_projects/praProject1/PB/pb/content_pb"
	"go_projects/praProject1/cmd/gateway/client"
	"go_projects/praProject1/cmd/gateway/middleware"
	"go_projects/praProject1/pkg/errcode"
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

// createPostReq 创建帖子的请求体。
//
// type 为 0（POST_TYPE_UNSPECIFIED）时由 gin validator 拦截。
// 失物招领 / 二手交易的扩展字段以嵌套对象传入；服务端会按 type 校验必填字段。
type createPostReq struct {
	Type       int32    `json:"type" binding:"required,min=1,max=3"`
	Title      string   `json:"title" binding:"required,min=1,max=200"`
	Content    string   `json:"content" binding:"required,min=1,max=5000"`
	Images     []string `json:"images"`
	LostFound  *lostFoundDTO  `json:"lost_found,omitempty"`
	SecondHand *secondHandDTO `json:"second_hand,omitempty"`
}

type lostFoundDTO struct {
	Location    string `json:"location"`
	Category    int32  `json:"category"`
	Contact     string `json:"contact"`
	LostOrFound int32  `json:"lost_or_found"`
}

type secondHandDTO struct {
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	Condition     int32   `json:"condition"`
	TradeMethod   int32   `json:"trade_method"`
	Category      int32   `json:"category"`
}

// updatePostReq 更新帖子请求体；标题/正文/图片任一变化即可，至少传一个字段。
type updatePostReq struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Images     []string `json:"images"`
	LostFound  *lostFoundDTO  `json:"lost_found,omitempty"`
	SecondHand *secondHandDTO `json:"second_hand,omitempty"`
}

// createCommentReq 创建一级评论。
type createCommentReq struct {
	PostID  int64  `json:"post_id" binding:"required,min=1"`
	Content string `json:"content" binding:"required,min=1,max=500"`
}

// searchReq 关键词搜索请求；keyword 必填，page/page_size 有默认值。
type searchReq struct {
	Keyword  string `json:"keyword" binding:"required,min=1"`
	Type     int32  `json:"type"`
	Category int32  `json:"category"`
	Page     int32  `json:"page"`
	PageSize int32  `json:"page_size"`
	Sort     int32  `json:"sort"`
}

// ─── POST /api/v1/content/posts  (JWT + school bound) ────────────────────────

// CreatePost 创建帖子。
//
// 强依赖 JWT 注入的 user_id 与 school_id；服务端基于 type 做 DFA 敏感词扫描。
func CreatePost(c *gin.Context) {
	var req createPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.ContentClient.CreatePost(ctx, &content_pb.CreatePostRequest{
		SchoolId:   sid,
		UserId:     uid,
		Type:       content_pb.PostType(req.Type),
		Title:      req.Title,
		Content:    req.Content,
		Images:     req.Images,
		LostFound:  dtoToLostFound(req.LostFound),
		SecondHand: dtoToSecondHand(req.SecondHand),
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"post_id":    resp.PostId,
		"status":     resp.Status,
		"created_at": resp.CreatedAt,
	})
}

// ─── GET /api/v1/content/posts  (JWT) ────────────────────────────────────────

// ListPosts 帖子列表（游标分页 + 多维筛选）。
//
// 读接口不强制 school 绑定；学校 ID 从 JWT 注入（未绑定时 sid=0，
// 服务端会基于 sid=0 返回跨学校内容，由后续业务决定是否需前端引导绑定）。
func ListPosts(c *gin.Context) {
	ctx, sid, _, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	pagination := &common_pb.CursorPaginationReq{
		Cursor:   c.Query("cursor"),
		PageSize: int32(parseQueryInt(c, "page_size", 20)),
	}
	status := parseQueryInt(c, "status", int(content_pb.PostStatus_POST_STATUS_PUBLISHED))

	resp, err := client.ContentClient.ListPosts(ctx, &content_pb.ListPostsRequest{
		SchoolId:   sid,
		Type:       content_pb.PostType(parseQueryInt(c, "type", 0)),
		Category:   content_pb.ItemCategory(parseQueryInt(c, "category", 0)),
		Status:     content_pb.PostStatus(status),
		Pagination: pagination,
		Sort:       common_pb.SortType(parseQueryInt(c, "sort", 0)),
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"posts":       resp.Posts,
		"next_cursor": resp.NextCursor,
		"has_more":    resp.HasMore,
	})
}

// ─── GET /api/v1/content/posts/:id  (JWT) ────────────────────────────────────

// GetPost 获取帖子详情（含 is_liked / is_owner）。
func GetPost(c *gin.Context) {
	ctx, sid, uid, ok := readCtxWithIDs(c)
	if !ok {
		return
	}
	postID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	resp, err := client.ContentClient.GetPost(ctx, &content_pb.GetPostRequest{
		SchoolId:     sid,
		PostId:       postID,
		ViewerUserId: uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"post":     resp.Post,
		"is_liked": resp.IsLiked,
		"is_owner": resp.IsOwner,
	})
}

// ─── PUT /api/v1/content/posts/:id  (JWT + school bound) ─────────────────────

// UpdatePost 编辑帖子（仅作者本人，服务端鉴权）。
func UpdatePost(c *gin.Context) {
	var req updatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}
	if req.Title == "" && req.Content == "" && len(req.Images) == 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "至少需要更新标题/正文/图片之一")
		return
	}

	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}
	postID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	_, err := client.ContentClient.UpdatePost(ctx, &content_pb.UpdatePostRequest{
		SchoolId:   sid,
		PostId:     postID,
		UserId:     uid,
		Title:      req.Title,
		Content:    req.Content,
		Images:     req.Images,
		LostFound:  dtoToLostFound(req.LostFound),
		SecondHand: dtoToSecondHand(req.SecondHand),
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ─── DELETE /api/v1/content/posts/:id  (JWT + school bound) ──────────────────

// DeletePost 软删除帖子（仅作者或管理员，服务端鉴权）。
func DeletePost(c *gin.Context) {
	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}
	postID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	_, err := client.ContentClient.DeletePost(ctx, &content_pb.DeletePostRequest{
		SchoolId: sid,
		PostId:   postID,
		UserId:   uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ─── POST /api/v1/content/comments  (JWT + school bound) ─────────────────────

// CreateComment 创建一级评论。
func CreateComment(c *gin.Context) {
	var req createCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}
	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}

	resp, err := client.ContentClient.CreateComment(ctx, &content_pb.CreateCommentRequest{
		SchoolId: sid,
		PostId:   req.PostID,
		UserId:   uid,
		Content:  req.Content,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"comment_id": resp.CommentId,
		"created_at": resp.CreatedAt,
	})
}

// ─── DELETE /api/v1/content/comments/:id  (JWT + school bound) ───────────────

// DeleteComment 软删除评论（仅作者本人）。
func DeleteComment(c *gin.Context) {
	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}
	commentID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	_, err := client.ContentClient.DeleteComment(ctx, &content_pb.DeleteCommentRequest{
		SchoolId:  sid,
		CommentId: commentID,
		UserId:    uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ─── GET /api/v1/content/posts/:id/comments  (JWT) ───────────────────────────

// ListComments 评论列表（游标分页）。
func ListComments(c *gin.Context) {
	ctx, sid, _, ok := readCtxWithIDs(c)
	if !ok {
		return
	}
	postID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	pagination := &common_pb.CursorPaginationReq{
		Cursor:   c.Query("cursor"),
		PageSize: int32(parseQueryInt(c, "page_size", 20)),
	}
	resp, err := client.ContentClient.ListComments(ctx, &content_pb.ListCommentsRequest{
		SchoolId:   sid,
		PostId:     postID,
		Pagination: pagination,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"comments":    resp.Comments,
		"next_cursor": resp.NextCursor,
		"has_more":    resp.HasMore,
	})
}

// ─── POST /api/v1/content/posts/:id/like  (JWT + school bound) ───────────────

// LikePost 点赞帖子。
func LikePost(c *gin.Context) {
	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}
	postID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	resp, err := client.ContentClient.LikePost(ctx, &content_pb.LikePostRequest{
		SchoolId: sid,
		PostId:   postID,
		UserId:   uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"liked":       resp.Liked,
		"likes_count": resp.LikesCount,
	})
}

// ─── DELETE /api/v1/content/posts/:id/like  (JWT + school bound) ────────────

// UnlikePost 取消点赞。
func UnlikePost(c *gin.Context) {
	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return
	}
	postID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}

	resp, err := client.ContentClient.UnlikePost(ctx, &content_pb.UnlikePostRequest{
		SchoolId: sid,
		PostId:   postID,
		UserId:   uid,
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"liked":       resp.Liked,
		"likes_count": resp.LikesCount,
	})
}

// ─── GET /api/v1/content/search  (JWT) ───────────────────────────────────────

// SearchContent 关键词搜索（代理 ES 查询）。
//
// 与 ListPosts 的区别：本接口走 ES（按相关度/时间排序），用于全局搜索；
// ListPosts 走 DB（按时间/点赞数排序），用于浏览列表。
func SearchContent(c *gin.Context) {
	var req searchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}
	ctx, sid, _, ok := readCtxWithIDs(c)
	if !ok {
		return
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	resp, err := client.ContentClient.SearchContent(ctx, &content_pb.SearchContentRequest{
		SchoolId: sid,
		Keyword:  req.Keyword,
		Type:     content_pb.PostType(req.Type),
		Category: content_pb.ItemCategory(req.Category),
		Status:   content_pb.PostStatus_POST_STATUS_PUBLISHED,
		Page:     page,
		PageSize: pageSize,
		Sort:     common_pb.SortType(req.Sort),
	})
	if err != nil {
		middleware.GRPCErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"posts":     resp.Posts,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
		"took_ms":   resp.TookMs,
	})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// authCtxWithIDs 写接口上下文：必须 JWT + 学校绑定（RequireSchoolBound 中间件已校验）。
//
// 返回 outbound gRPC context（已注入 user-id / user-role / school-id metadata，
// 供 Content Service 通过 metadata.FromIncomingContext 读取）。
func authCtxWithIDs(c *gin.Context) (context.Context, int64, int64, bool) {
	ctx, ok := authCtx(c)
	if !ok {
		return nil, 0, 0, false
	}
	uid, _ := userID(c)
	sid, _ := schoolID(c)
	if uid == 0 {
		middleware.ErrorResponse(c, errcode.ErrMissingToken, "缺少用户身份")
		return nil, 0, 0, false
	}
	return ctx, uid, sid, true
}

// readCtxWithIDs 读接口上下文：仅需 JWT，不强制 school 绑定。
//
// 仍构造 outbound metadata 让下游能拿到 viewer_user_id / school_id；
// 即使 sid=0 也照常转发，由服务端按业务策略决定是否做学校隔离。
func readCtxWithIDs(c *gin.Context) (context.Context, int64, int64, bool) {
	ctx, uid, sid, ok := authCtxWithIDs(c)
	if !ok {
		return nil, 0, 0, false
	}
	return ctx, sid, uid, true
}

// userID 从 gin.Context 读 user_id；JWT 中间件保证已注入。
func userID(c *gin.Context) (int64, bool) {
	v, exists := c.Get(middleware.CtxUserID)
	if !exists {
		return 0, false
	}
	uid, ok := v.(int64)
	return uid, ok
}

// schoolID 从 gin.Context 读 school_id；未绑定时为 0。
func schoolID(c *gin.Context) (int64, bool) {
	v, exists := c.Get(middleware.CtxSchoolID)
	if !exists {
		return 0, false
	}
	sid, ok := v.(int64)
	return sid, ok
}

// dtoToLostFound 把 gateway 层 DTO 转成 protobuf 扩展字段。
// nil 输入返回 nil，服务端会按 type 判断是否必填。
func dtoToLostFound(d *lostFoundDTO) *content_pb.LostFoundExtra {
	if d == nil {
		return nil
	}
	return &content_pb.LostFoundExtra{
		Location:     d.Location,
		ItemCategory: content_pb.ItemCategory(d.Category),
		Contact:      d.Contact,
		LostOrFound:  content_pb.LostOrFoundType(d.LostOrFound),
	}
}

// dtoToSecondHand 把 gateway 层 DTO 转成 protobuf 扩展字段。
func dtoToSecondHand(d *secondHandDTO) *content_pb.SecondHandExtra {
	if d == nil {
		return nil
	}
	return &content_pb.SecondHandExtra{
		Price:         d.Price,
		OriginalPrice: d.OriginalPrice,
		Condition:     content_pb.Condition(d.Condition),
		TradeMethod:   content_pb.TradeMethod(d.TradeMethod),
		ItemCategory:  content_pb.ItemCategory(d.Category),
	}
}

// parsePathInt64 解析路径参数为 int64；解析失败时已写入统一错误响应。
func parsePathInt64(c *gin.Context, key string) (int64, bool) {
	v := c.Param(key)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		middleware.ErrorResponse(c, errcode.ErrInvalidParam, "路径参数 "+key+" 无效")
		return 0, false
	}
	return n, true
}

// parseQueryInt 解析 query 参数为 int，缺省返回 defVal。
func parseQueryInt(c *gin.Context, key string, defVal int) int {
	v := c.Query(key)
	if v == "" {
		return defVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defVal
	}
	return n
}