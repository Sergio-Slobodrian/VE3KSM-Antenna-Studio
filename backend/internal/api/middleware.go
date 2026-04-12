package api

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
)

// SetupCORS returns a Gin middleware handler that applies a CORS policy
// allowing the specified origins. This is required because the React frontend
// (typically on localhost:5173) makes cross-origin requests to the Go backend
// (typically on localhost:8080).
//
// The middleware handles preflight OPTIONS requests by returning 204 with the
// appropriate Access-Control-Allow-* headers and aborting the chain (no handler
// invocation). Non-preflight requests pass through with CORS headers attached.
//
// MaxAge of 86400 seconds (24 hours) tells browsers to cache the preflight
// response, reducing the number of OPTIONS round-trips during development.
func SetupCORS(allowedOrigins []string) gin.HandlerFunc {
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours preflight cache
	})

	return func(ctx *gin.Context) {
		c.HandlerFunc(ctx.Writer, ctx.Request)
		// For preflight requests, return immediately without invoking handlers
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
			return
		}
		ctx.Next()
	}
}

// RequestLogger returns a Gin middleware that logs every completed request.
// It records the HTTP status code, method, path, and wall-clock duration.
// Logging happens after the handler chain completes (post-c.Next()) so the
// status code reflects the actual response.
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

// Recovery returns Gin's built-in panic recovery middleware. If any handler
// in the chain panics, this middleware catches it, logs the stack trace, and
// returns a 500 Internal Server Error instead of crashing the server process.
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
