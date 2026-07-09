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

	// User Service – schools route（游客可搜索学校列表，用于绑定学校）
	schools := v1.Group("/schools", middleware.OptionalJWTAuth())
	{
		schools.GET("", handler.ListSchools)
	}

	// User Service – public routes
	userPublic := v1.Group("/user")
	{
		userPublic.POST("/login", handler.WxLogin)        // WeChat login → 双 Token
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

	// Task Service – routes (Issue #66, 游客模式)
	// 读路由：游客可浏览任务列表和详情
	tasksRead := v1.Group("/tasks", middleware.OptionalJWTAuth())
	{
		tasksRead.GET("", handler.ListTasks)
		tasksRead.GET("/:id", handler.GetTask)
	}
	// 写路由：JWT + 学校绑定
	tasksWrite := v1.Group("/tasks", middleware.JWTAuth(), middleware.RequireSchoolBound())
	{
		tasksWrite.POST("", handler.CreateTask)
		tasksWrite.PUT("/:id", handler.UpdateTask)
		tasksWrite.DELETE("/:id", handler.DeleteTask)
		tasksWrite.POST("/:id/claim", handler.ClaimTask)
		tasksWrite.PUT("/:id/complete", handler.CompleteTask)
		tasksWrite.PUT("/:id/cancel", handler.CancelTask)
	}

	// File Service – routes (Issue #79, 游客模式)
	// 读路由：游客可查看文件元数据
	filesRead := v1.Group("/files", middleware.OptionalJWTAuth())
	{
		filesRead.GET("/:id", handler.GetFile)
	}
	// 写路由：JWT 鉴权（上传不需要学校绑定）
	filesWrite := v1.Group("/files", middleware.JWTAuth())
	{
		filesWrite.POST("/upload", handler.UploadFile)

		// 删除：JWT + 学校绑定
		delete := filesWrite.Group("", middleware.RequireSchoolBound())
		{
			delete.DELETE("/:id", handler.DeleteFile)
		}
	}

	// Admin Service – v2.0 管理员路由组 (Issue #85)
	//   - admin 级别可访问封禁/解封/用户列表/内容审核
	//   - super_admin 级别额外可访问角色设置
	admin := v1.Group("/admin", middleware.JWTAuth())
	admin.Use(middleware.RequireRole(2)) // RoleAdmin = 2
	{
		admin.POST("/users/ban", handler.AdminBanUser)
		admin.POST("/users/unban", handler.AdminUnbanUser)
		admin.GET("/users/list", handler.AdminListUsers)
		admin.GET("/content/audit-list", handler.AdminListContentForAudit)
		admin.POST("/content/audit", handler.AdminAuditContent)
	}
	// SetUserRole 需要 super_admin
	superAdminRole := v1.Group("/admin/users", middleware.JWTAuth())
	superAdminRole.Use(middleware.RequireRole(3)) // RoleSuperAdmin = 3
	{
		superAdminRole.POST("/set-role", handler.AdminSetUserRole)
	}

	// Content Service – routes (Issue #22, #41, 游客模式)
	//   读路由（List/Get/Search/ListComments）：游客可浏览
	//   写路由（Create/Update/Delete/Like/Comment）：JWT + RequireSchoolBound
	contentRead := v1.Group("/content", middleware.OptionalJWTAuth())
	{
		contentRead.GET("/posts", handler.ListPosts)
		contentRead.GET("/posts/:id", handler.GetPost)
		contentRead.GET("/posts/:id/comments", handler.ListComments)
		contentRead.GET("/comments/:id/replies", handler.ListCommentReplies)
		contentRead.POST("/search", handler.SearchContent)
	}

	contentWrite := v1.Group("/content", middleware.JWTAuth(), middleware.RequireSchoolBound())
	{
		contentWrite.POST("/posts", handler.CreatePost)
		contentWrite.PUT("/posts/:id", handler.UpdatePost)
		contentWrite.DELETE("/posts/:id", handler.DeletePost)
		contentWrite.POST("/posts/:id/like", handler.LikePost)
		contentWrite.DELETE("/posts/:id/like", handler.UnlikePost)
		contentWrite.POST("/comments", handler.CreateComment)
		contentWrite.DELETE("/comments/:id", handler.DeleteComment)
	}

	return r
}

// Router kept for backward compatibility with cmd/main.go.
func Router() *gin.Engine { return NewRouter() }
