package assets

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register wires the asset routes into a Gin engine:
//
//	GET /assets/app.js   - bundled JavaScript
//	GET /assets/app.css  - bundled CSS (if any)
//	GET /*any            - index.html (SPA fallback, excluding /api)
//
// The API routes should be registered BEFORE calling Register so that
// the NoRoute SPA handler only catches unmatched paths.
func (b *Bundler) Register(r *gin.Engine) {
	r.GET("/assets/app.js", func(c *gin.Context) {
		bundle, err := b.Get()
		if err != nil {
			log.Printf("assets: rebuild failed: %v", err)
			c.String(http.StatusInternalServerError, "bundle error: %v", err)
			return
		}
		c.Header("Cache-Control", cacheControl(b.opts.Dev))
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", bundle.JS)
	})

	r.GET("/assets/app.css", func(c *gin.Context) {
		bundle, err := b.Get()
		if err != nil {
			c.String(http.StatusInternalServerError, "bundle error: %v", err)
			return
		}
		if len(bundle.CSS) == 0 {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", cacheControl(b.opts.Dev))
		c.Data(http.StatusOK, "text/css; charset=utf-8", bundle.CSS)
	})

	// SPA fallback.  Any request that didn't hit /api/* or /assets/*
	// returns the rewritten index.html so client-side routing works.
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}
		bundle, err := b.Get()
		if err != nil {
			c.String(http.StatusInternalServerError, "bundle error: %v", err)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", bundle.HTML)
	})
}

func cacheControl(dev bool) string {
	if dev {
		return "no-cache"
	}
	// Bundle contents change between builds but the URL is stable,
	// so a short max-age plus revalidation strikes a reasonable balance
	// without the overhead of content-hashed filenames.
	return "public, max-age=60, must-revalidate"
}
