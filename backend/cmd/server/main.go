// Package main is the single entry point for the Antenna Studio server.
//
// One Go binary does everything:
//
//   - Serves the JSON API under /api/*  (MoM solver, templates, sweeps).
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

	// Build the bundle eagerly so a broken frontend fails the process
	// before it starts accepting traffic.
	bundler, err := assets.NewBundler(assets.Options{
		FrontendDir: frontendDir,
		Dev:         *devFlag,
	})
	if err != nil {
		log.Fatalf("frontend bundle failed: %v", err)
	}
	log.Printf("frontend bundled from %s (dev=%v)", frontendDir, *devFlag)

	// gin.Default() includes Gin's built-in logger and recovery middleware.
	router := gin.Default()

	// CORS is still registered so the API remains usable from a separate
	// origin if someone runs a bespoke client — but in the normal
	// deployment everything is same-origin and the header is a no-op.
	router.Use(api.SetupCORS(cfg.CORSOrigins))
	router.Use(api.RequestLogger())

	// API routes — registered before assets so /api/* wins over the SPA
	// fallback installed by bundler.Register.
	router.POST("/api/simulate", api.HandleSimulate)
	router.POST("/api/sweep", api.HandleSweep)
	router.GET("/api/templates", api.HandleGetTemplates)
	router.POST("/api/templates/:name", api.HandleGenerateTemplate)

	bundler.Register(router)

	addr := ":" + cfg.Port
	log.Printf("Antenna Studio listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// detectFrontendDir walks up from the current working directory looking
// for a folder that contains index.html + src/main.tsx.  This lets
// `go run ./cmd/server` Just Work from either the repo root or backend/.
func detectFrontendDir() string {
	candidates := []string{
		"frontend",
		"../frontend",
		"../../frontend",
	}
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
