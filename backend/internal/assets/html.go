package assets

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// scriptRegex locates the original Vite entry `<script type="module"
// src="/src/main.tsx"></script>` so we can swap it for the bundled
// output.  The surrounding whitespace is preserved.
var scriptRegex = regexp.MustCompile(`(?is)<script[^>]*src="/src/main\.tsx"[^>]*></script>`)

// headClose matches the first </head> (case-insensitive) so we can inject
// a stylesheet <link> into it when the bundle produced CSS.
var headClose = regexp.MustCompile(`(?i)</head>`)

// renderIndexHTML reads frontend/index.html and rewrites the Vite-era
// <script src="/src/main.tsx"> reference to point at the compiled bundle
// served by the Go backend at /assets/app.js (+ /assets/app.css).
func renderIndexHTML(frontendDir string, hasCSS bool) ([]byte, error) {
	path := filepath.Join(frontendDir, "index.html")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("assets: read %s: %w", path, err)
	}

	replacement := `<script type="module" src="/assets/app.js"></script>`
	out := scriptRegex.ReplaceAll(raw, []byte(replacement))
	if bytes.Equal(out, raw) {
		// Fallback: no <script> tag found (custom index.html); append
		// to </body> so the page still boots.
		bodyClose := regexp.MustCompile(`(?i)</body>`)
		out = bodyClose.ReplaceAll(out, []byte(replacement+"\n</body>"))
	}

	if hasCSS {
		link := `<link rel="stylesheet" href="/assets/app.css">` + "\n"
		out = headClose.ReplaceAll(out, []byte(link+"</head>"))
	}

	return out, nil
}
