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

// Package config handles server configuration for the VE3KSM Antenna Studio backend.
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
