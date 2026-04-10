package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
)

// SetupCORS returns a Gin middleware handler that applies CORS policy
// for the specified allowed origins.
func SetupCORS(allowedOrigins []string) gin.HandlerFunc {
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400,
	})

	return func(ctx *gin.Context) {
		c.HandlerFunc(ctx.Writer, ctx.Request)
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
			return
		}
		ctx.Next()
	}
}

// RequestLogger logs each incoming request with method, path, status, and duration.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		log.Printf("[%d] %s %s (%v)", status, c.Request.Method, path, duration)
	}
}

// Recovery returns Gin's built-in recovery middleware that catches panics
// and returns a 500 error.
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
