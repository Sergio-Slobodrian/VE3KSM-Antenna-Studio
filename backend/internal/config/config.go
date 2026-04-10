package config

import (
	"os"
	"strings"
)

// Config holds server configuration.
type Config struct {
	Port        string
	CORSOrigins []string
}

// Load reads configuration from environment variables with sensible defaults.
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
		origins = []string{
			"http://localhost:5173",
			"http://localhost:3000",
		}
	}

	return Config{
		Port:        port,
		CORSOrigins: origins,
	}
}
