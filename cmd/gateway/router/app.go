package router

import (
	"github.com/gin-gonic/gin"

	"go_projects/praProject1/cmd/gateway/handler"
	"go_projects/praProject1/cmd/gateway/middleware"
)

// NewRouter builds the Gin engine with all middleware and routes.
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
	user := v1.Group("/user")
	{
		user.POST("/login", handler.WxLogin) // WeChat login → JWT
	}

	// User Service – authenticated routes
	auth := v1.Group("/user", middleware.JWTAuth())
	{
		auth.GET("/me", handler.GetCurrentUser)     // get own profile
		auth.PUT("/info", handler.UpdateUserInfo)   // update nickname / avatar
		auth.PUT("/campus", handler.BindCampus)     // bind school
	}

	return r
}

// Router kept for backward compatibility with cmd/main.go.
func Router() *gin.Engine { return NewRouter() }