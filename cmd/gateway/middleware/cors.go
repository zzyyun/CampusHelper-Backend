package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 跨域资源共享 前端能够跨域调用后端接口.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许访问接口的前端域名
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID,X-Trace-ID")
		c.Header("Access-Control-Expose-Headers", "X-Trace-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
