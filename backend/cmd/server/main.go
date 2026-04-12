// Package main is the entry point for the Antenna Studio backend server.
// It loads configuration from environment variables, sets up the Gin HTTP
// router with CORS and logging middleware, registers API routes, and starts
// listening. The server can be run standalone or via the launcher
// (cmd/launcher) which also starts the frontend dev server.
package main

import (
	"log"

	"antenna-studio/backend/internal/api"
	"antenna-studio/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// gin.Default() includes Gin's built-in logger and recovery middleware
	router := gin.Default()

	// CORS must be registered before route handlers so that preflight OPTIONS
	// requests are handled correctly. RequestLogger runs after handlers complete.
	router.Use(api.SetupCORS(cfg.CORSOrigins))
	router.Use(api.RequestLogger())

	// API routes:
	//   POST /api/simulate          - Run a single-frequency MoM simulation
	//   POST /api/sweep             - Run a frequency sweep (SWR/impedance curves)
	//   GET  /api/templates         - List available antenna preset templates
	//   POST /api/templates/:name   - Generate geometry from a named template
	router.POST("/api/simulate", api.HandleSimulate)
	router.POST("/api/sweep", api.HandleSweep)
	router.GET("/api/templates", api.HandleGetTemplates)
	router.POST("/api/templates/:name", api.HandleGenerateTemplate)

	addr := ":" + cfg.Port
	log.Printf("Antenna Studio backend starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
