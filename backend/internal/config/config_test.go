package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Ensure env vars are unset so defaults apply.
	os.Unsetenv("PORT")
	os.Unsetenv("CORS_ORIGINS")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Port)
	}

	if len(cfg.CORSOrigins) != 2 {
		t.Fatalf("expected 2 default CORS origins, got %d", len(cfg.CORSOrigins))
	}

	want := map[string]bool{
		"http://localhost:5173": true,
		"http://localhost:3000": true,
	}
	for _, o := range cfg.CORSOrigins {
		if !want[o] {
			t.Fatalf("unexpected default CORS origin: %q", o)
		}
	}
}

func TestLoad_PortFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	t.Cleanup(func() { os.Unsetenv("PORT") })

	cfg := Load()

	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090 from env, got %q", cfg.Port)
	}
}

func TestLoad_CORSOriginsFromEnv(t *testing.T) {
	os.Setenv("CORS_ORIGINS", "https://example.com , https://other.dev")
	t.Cleanup(func() { os.Unsetenv("CORS_ORIGINS") })

	cfg := Load()

	if len(cfg.CORSOrigins) != 2 {
		t.Fatalf("expected 2 CORS origins, got %d: %v", len(cfg.CORSOrigins), cfg.CORSOrigins)
	}
	if cfg.CORSOrigins[0] != "https://example.com" {
		t.Fatalf("expected first origin trimmed to 'https://example.com', got %q", cfg.CORSOrigins[0])
	}
	if cfg.CORSOrigins[1] != "https://other.dev" {
		t.Fatalf("expected second origin trimmed to 'https://other.dev', got %q", cfg.CORSOrigins[1])
	}
}
