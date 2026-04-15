// Package config handles server configuration for the Antenna Studio backend.
// All settings are read from environment variables, with sensible defaults
// for local development. The launcher (cmd/launcher) sets these variables
// when spawning the backend process.
package config

import (
	"os"
	"strings"
)

// Config holds the runtime configuration for the backend HTTP server.
type Config struct {
	// Port is the TCP port the server listens on (e.g. "8080").
	Port string
	// CORSOrigins is the list of allowed origins for cross-origin requests.
	// The frontend origin must be in this list or API calls will be blocked.
	CORSOrigins []string
}

// Load reads configuration from environment variables with sensible defaults
// for local development:
//   - PORT: defaults to "8080" if not set.
//   - CORS_ORIGINS: comma-separated list of allowed origins. Defaults to
//     "http://localhost:5173" (Vite dev server) and "http://localhost:3000"
//     (alternative dev port). Leading/trailing whitespace around each origin
//     is trimmed to tolerate "origin1, origin2" formatting.
func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	corsOrigins := os.Getenv("CORS_ORIGINS")
	var origins []string
	if corsOrigins != "" {
		origins = strings.Split(corsOrigins, ",")
		for i, o := range origins {
			origins[i] = strings.TrimSpace(o)
		}
	} else {
		// Default origins cover the two most common local dev server ports
		origins = []string{
			"http://localhost:5173", // Vite default
			"http://localhost:3000", // CRA / alternative
		}
	}

	return Config{
		Port:        port,
		CORSOrigins: origins,
	}
}
