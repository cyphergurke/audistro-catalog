package config

import "testing"

func TestLoadFromEnvDefaultsToProd(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "")
	t.Setenv("CATALOG_ENV", "")
	t.Setenv("CATALOG_ALLOW_INSECURE_TRANSPORT", "")

	cfg := LoadFromEnv()
	if cfg.Env != "prod" {
		t.Fatalf("expected default env=prod, got %q", cfg.Env)
	}
	if cfg.IsInsecureTransportAllowed() {
		t.Fatalf("expected insecure transport disabled by default")
	}
}

func TestLoadFromEnvSupportsCatalogEnvDev(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "")
	t.Setenv("CATALOG_ENV", "dev")
	t.Setenv("CATALOG_ALLOW_INSECURE_TRANSPORT", "")

	cfg := LoadFromEnv()
	if cfg.Env != "dev" {
		t.Fatalf("expected env=dev, got %q", cfg.Env)
	}
	if !cfg.IsInsecureTransportAllowed() {
		t.Fatalf("expected insecure transport enabled in dev")
	}
}

func TestLoadFromEnvUnknownFallsBackToProd(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "qa")
	t.Setenv("CATALOG_ENV", "")
	t.Setenv("CATALOG_ALLOW_INSECURE_TRANSPORT", "")

	cfg := LoadFromEnv()
	if cfg.Env != "prod" {
		t.Fatalf("expected unknown env to fallback to prod, got %q", cfg.Env)
	}
	if cfg.IsInsecureTransportAllowed() {
		t.Fatalf("expected insecure transport disabled for unknown env fallback")
	}
}

func TestLoadFromEnvAllowsInsecureTransportFlagInProd(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "prod")
	t.Setenv("CATALOG_ENV", "")
	t.Setenv("CATALOG_ALLOW_INSECURE_TRANSPORT", "true")

	cfg := LoadFromEnv()
	if cfg.Env != "prod" {
		t.Fatalf("expected env=prod, got %q", cfg.Env)
	}
	if !cfg.IsInsecureTransportAllowed() {
		t.Fatalf("expected insecure transport enabled via CATALOG_ALLOW_INSECURE_TRANSPORT")
	}
}
