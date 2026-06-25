package router

import (
	"github.com/gin-gonic/gin"

	"go_projects/praProject1/cmd/gateway/handler"
	"go_projects/praProject1/cmd/gateway/middleware"
)

// NewRouter 构造 Gin 引擎，注册全局中间件与路由。
//
// 中间件挂载策略（按 Issue #19）：
//   - 全局：CORS / RateLimit / Trace
//   - 鉴权：JWT（需登录的接口）
//   - 校园绑定：RequireSchoolBound（写接口，未绑定学校用户拒绝，但 BindCampus 除外）
func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// ── Global middleware ──────────────────────────────────────────────────
	r.Use(middleware.CORS())
	r.Use(middleware.RateLimit())
	r.Use(middleware.Trace())

	// ── Health check ───────────────────────────────────────────────────────
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// ── API v1 ─────────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")

	// User Service – public routes
	userPublic := v1.Group("/user")
	{
		userPublic.POST("/login", handler.WxLogin)   // WeChat login → 双 Token
		userPublic.POST("/refresh", handler.RefreshToken) // Refresh Token → 新 Access Token
	}

	// User Service – authenticated routes
	//   - GET /me           读，未绑定也可调用
	//   - PUT /campus       绑定学校本身，不要求已绑定（绑定完成后才受 RequireSchoolBound 约束）
	//   - PUT /info         写，要求已绑定学校
	auth := v1.Group("/user", middleware.JWTAuth())
	{
		auth.GET("/me", handler.GetCurrentUser)
		auth.PUT("/campus", handler.BindCampus)

		// 写路由组：JWT + 校园绑定
		write := auth.Group("", middleware.RequireSchoolBound())
		{
			write.PUT("/info", handler.UpdateUserInfo) // update nickname / avatar
		}
	}

		// Message Service – notification routes (Issue #46)
		// 所有通知 API 仅需 JWT 鉴权，不需要 school 绑定（未绑定学校用户也可查看通知）
		notifications := v1.Group("/notifications", middleware.JWTAuth())
		{
			notifications.GET("", handler.ListNotifications)
			notifications.GET("/unread-count", handler.UnreadCount)
			notifications.PUT("/:id/read", handler.MarkRead)
			notifications.PUT("/read-all", handler.MarkAllRead)
			notifications.DELETE("/:id", handler.DeleteNotification)
		}

	// Content Service – authenticated routes (Issue #22, #41)
	//   12 个接口：帖子 CRUD / 评论 / 回复 / 点赞 / 搜索
	//   - 读路由（List/Get/Search/ListComments）：仅 JWT，未绑定学校也能浏览
	//   - 写路由（Create/Update/Delete/Like/Comment）：JWT + RequireSchoolBound
	content := v1.Group("/content", middleware.JWTAuth())
	{
		// 读
		content.GET("/posts", handler.ListPosts)                    // 列表
		content.GET("/posts/:id", handler.GetPost)                  // 详情
		content.GET("/posts/:id/comments", handler.ListComments)    // 评论列表
		content.GET("/comments/:id/replies", handler.ListCommentReplies) // 回复列表
		content.POST("/search", handler.SearchContent)              // 关键词搜索

		// 写：JWT + 学校绑定
		write := content.Group("", middleware.RequireSchoolBound())
		{
			write.POST("/posts", handler.CreatePost)                       // 发帖
			write.PUT("/posts/:id", handler.UpdatePost)                    // 编辑
			write.DELETE("/posts/:id", handler.DeletePost)                  // 删帖
			write.POST("/posts/:id/like", handler.LikePost)                // 点赞
			write.DELETE("/posts/:id/like", handler.UnlikePost)             // 取消点赞
			write.POST("/comments", handler.CreateComment)                  // 写评论
			write.DELETE("/comments/:id", handler.DeleteComment)            // 删评论
		}
	}

	return r
}

// Router kept for backward compatibility with cmd/main.go.
func Router() *gin.Engine { return NewRouter() }