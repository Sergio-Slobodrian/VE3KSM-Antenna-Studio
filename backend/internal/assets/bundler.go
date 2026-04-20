// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package assets bundles the TypeScript/TSX frontend source tree into
// browser-ready JavaScript and CSS entirely in-process using esbuild's
// Go API.  No Node.js runtime is required: TypeScript compilation runs
// in this Go backend before any bytes reach the browser.
//
// Typical lifecycle:
//
//	b, err := assets.NewBundler(assets.Options{FrontendDir: "../frontend"})
//	// first build happens eagerly, subsequent calls in dev mode can Rebuild()
//	http.Handle("/", b.Handler())
package assets

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
)

// Options configures a Bundler.  FrontendDir must contain an index.html
// alongside a src/ directory with main.tsx as the entry point.  In Dev
// mode the bundle is re-built on every asset request; otherwise it is
// built once at construction time.
type Options struct {
	FrontendDir string // path to the frontend/ directory (has index.html + src/)
	Dev         bool   // if true, rebuild on every Get; otherwise build-once
	SourceMaps  bool   // emit inline source maps (dev convenience)
}

// Bundle is the in-memory result of a successful esbuild run.
type Bundle struct {
	HTML     []byte    // index.html, rewritten to reference /assets/app.js + /assets/app.css
	JS       []byte    // compiled, bundled JavaScript (main.tsx + transitive imports)
	CSS      []byte    // compiled, bundled CSS (imports pulled out of JS)
	BuiltAt  time.Time // timestamp of the successful build
	Duration time.Duration
}

// Bundler compiles the frontend TS/TSX/CSS tree into a Bundle.  It is
// safe for concurrent use: Get() returns the cached bundle (or rebuilds
// if Dev mode is on).
type Bundler struct {
	opts   Options
	mu     sync.RWMutex
	cached *Bundle
}

// NewBundler validates the frontend directory layout, performs the first
// build eagerly, and returns a ready-to-serve Bundler.
func NewBundler(opts Options) (*Bundler, error) {
	if opts.FrontendDir == "" {
		return nil, fmt.Errorf("assets: FrontendDir is required")
	}
	abs, err := filepath.Abs(opts.FrontendDir)
	if err != nil {
		return nil, fmt.Errorf("assets: resolve frontend dir: %w", err)
	}
	opts.FrontendDir = abs

	b := &Bundler{opts: opts}
	if _, err := b.Rebuild(); err != nil {
		return nil, err
	}
	return b, nil
}

// Get returns the current bundle, rebuilding first if Dev mode is on.
func (b *Bundler) Get() (*Bundle, error) {
	if b.opts.Dev {
		return b.Rebuild()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cached, nil
}

// Rebuild compiles the frontend from disk and atomically replaces the
// cached bundle on success.  Safe to call while Get() is being served.
func (b *Bundler) Rebuild() (*Bundle, error) {
	start := time.Now()

	entry := filepath.Join(b.opts.FrontendDir, "src", "main.tsx")
	srcRoot := filepath.Join(b.opts.FrontendDir, "src")

	result := esbuild.Build(esbuild.BuildOptions{
		EntryPoints:       []string{entry},
		Bundle:            true,
		Write:             false,
		Outdir:            "/virtual", // never written; required so esbuild assigns output paths
		EntryNames:        "app",
		Format:            esbuild.FormatESModule,
		Target:            esbuild.ES2020,
		JSX:               esbuild.JSXAutomatic, // react-jsx (React 17+ transform)
		Sourcemap:         sourcemapMode(b.opts),
		MinifyWhitespace:  !b.opts.Dev,
		MinifyIdentifiers: !b.opts.Dev,
		MinifySyntax:      !b.opts.Dev,
		LogLevel:          esbuild.LogLevelSilent,
		Loader: map[string]esbuild.Loader{
			".ts":   esbuild.LoaderTS,
			".tsx":  esbuild.LoaderTSX,
			".js":   esbuild.LoaderJS,
			".jsx":  esbuild.LoaderJSX,
			".css":  esbuild.LoaderCSS,
			".json": esbuild.LoaderJSON,
			".svg":  esbuild.LoaderDataURL,
			".png":  esbuild.LoaderDataURL,
		},
		// Replace the Vite-specific import.meta.env reference with an
		// empty string literal: the Go server is same-origin so the
		// API client falls through to its relative-path default.
		Define: map[string]string{
			"import.meta.env.VITE_API_BASE": `""`,
			"process.env.NODE_ENV":          stringLit(nodeEnv(b.opts)),
		},
		// @/foo -> frontend/src/foo (mirrors the old tsconfig paths).
		ResolveExtensions: []string{".tsx", ".ts", ".jsx", ".js", ".json"},
		AbsWorkingDir:     b.opts.FrontendDir,
		NodePaths:         []string{filepath.Join(b.opts.FrontendDir, "node_modules")},
		Alias: map[string]string{
			"@": srcRoot,
		},
	})

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("assets: esbuild failed: %s", formatEsbuildErrors(result.Errors))
	}

	bundle := &Bundle{BuiltAt: time.Now(), Duration: time.Since(start)}
	for _, f := range result.OutputFiles {
		switch strings.ToLower(filepath.Ext(f.Path)) {
		case ".js":
			bundle.JS = f.Contents
		case ".css":
			bundle.CSS = f.Contents
		}
	}
	if len(bundle.JS) == 0 {
		return nil, fmt.Errorf("assets: esbuild produced no JS output")
	}

	html, err := renderIndexHTML(b.opts.FrontendDir, len(bundle.CSS) > 0)
	if err != nil {
		return nil, err
	}
	bundle.HTML = html

	b.mu.Lock()
	b.cached = bundle
	b.mu.Unlock()

	return bundle, nil
}

func sourcemapMode(o Options) esbuild.SourceMap {
	if o.SourceMaps || o.Dev {
		return esbuild.SourceMapInline
	}
	return esbuild.SourceMapNone
}

func nodeEnv(o Options) string {
	if o.Dev {
		return "development"
	}
	return "production"
}

func stringLit(s string) string {
	// JSON encoding of a string yields a valid JS string literal.
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func formatEsbuildErrors(errs []esbuild.Message) string {
	var sb strings.Builder
	for i, e := range errs {
		if i > 0 {
			sb.WriteString("\n")
		}
		if e.Location != nil {
			fmt.Fprintf(&sb, "%s:%d:%d: %s", e.Location.File, e.Location.Line, e.Location.Column, e.Text)
		} else {
			sb.WriteString(e.Text)
		}
	}
	return sb.String()
}
