// Package main is the single entry point for the VE3KSM Antenna Studio server.
//
// One Go binary does everything:
//
//   - Serves the JSON API under /api/*  (MoM solver, templates, sweeps,
//     NEC-2 import/export).
//   - Compiles the TypeScript/TSX frontend in-process via esbuild's Go
//     library and serves the resulting JS/CSS/HTML under /assets/* and /.
//
// There is no Node.js, Vite, or nginx in the runtime path.  TypeScript
// is transpiled inside this backend process before any byte is sent to
// the browser.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"antenna-studio/backend/internal/api"
	"antenna-studio/backend/internal/assets"
	"antenna-studio/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func main() {
	var (
		devFlag         = flag.Bool("dev", false, "rebuild frontend on every asset request")
		frontendDirFlag = flag.String("frontend-dir", "", "path to frontend/ (auto-detected if empty)")
	)
	flag.Parse()

	cfg := config.Load()

	frontendDir := *frontendDirFlag
	if frontendDir == "" {
		frontendDir = detectFrontendDir()
	}

	bundler, err := assets.NewBundler(assets.Options{
		FrontendDir: frontendDir,
		Dev:         *devFlag,
	})
	if err != nil {
		log.Fatalf("frontend bundle failed: %v", err)
	}
	log.Printf("frontend bundled from %s (dev=%v)", frontendDir, *devFlag)

	router := gin.Default()

	router.Use(api.SetupCORS(cfg.CORSOrigins))
	router.Use(api.RequestLogger())

	// API routes — registered before assets so /api/* wins over the SPA fallback.
	router.POST("/api/simulate", api.HandleSimulate)
	router.POST("/api/sweep", api.HandleSweep)
	router.GET("/api/templates", api.HandleGetTemplates)
	router.POST("/api/templates/:name", api.HandleGenerateTemplate)
	router.POST("/api/nec2/import", api.HandleNEC2Import)
	router.POST("/api/nec2/export", api.HandleNEC2Export)
	router.POST("/api/match", api.HandleMatch)
	router.POST("/api/nearfield", api.HandleNearField)
	router.POST("/api/cma", api.HandleCMA)
	router.POST("/api/optimize", api.HandleOptimize)
	router.POST("/api/pareto-optimize", api.HandleParetoOptimize)
	router.POST("/api/transient", api.HandleTransient)
	router.POST("/api/convergence", api.HandleConvergence)

	bundler.Register(router)

	addr := ":" + cfg.Port
	log.Printf("VE3KSM Antenna Studio listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// detectFrontendDir walks up from cwd looking for a folder containing
// index.html + src/main.tsx, so `go run ./cmd/server` works from either
// the repo root or backend/.
func detectFrontendDir() string {
	candidates := []string{"frontend", "../frontend", "../../frontend"}
	for _, c := range candidates {
		if looksLikeFrontend(c) {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	wd, _ := os.Getwd()
	return wd
}

func looksLikeFrontend(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main.tsx")); err != nil {
		return false
	}
	return true
}
