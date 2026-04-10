package main

import (
	"log"

	"antenna-studio/backend/internal/api"
	"antenna-studio/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	router := gin.Default()

	// Middleware
	router.Use(api.SetupCORS(cfg.CORSOrigins))
	router.Use(api.RequestLogger())

	// Routes
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
