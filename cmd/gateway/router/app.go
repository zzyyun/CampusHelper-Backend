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
		userPublic.POST("/login", handler.WxLogin) // WeChat login → JWT
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

	return r
}

// Router kept for backward compatibility with cmd/main.go.
func Router() *gin.Engine { return NewRouter() }