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

func TestLoadFromEnvIncludesDevUploadSettings(t *testing.T) {
	t.Setenv("CATALOG_ENV", "dev")
	t.Setenv("CATALOG_ADMIN_TOKEN", "dev-admin-token")
	t.Setenv("CATALOG_PROVIDER_TARGETS", "eu_1|http://localhost:18082|http://audistro-provider_eu_1:8080|/mnt/providers/eu_1,eu_2|http://localhost:18083|http://audistro-provider_eu_2:8080|/mnt/providers/eu_2")
	t.Setenv("CATALOG_STORAGE_PATH", "/var/lib/audistro-catalog")
	t.Setenv("CATALOG_FAP_INTERNAL_BASE_URL", "http://audistro-fap:8080")
	t.Setenv("FAP_PUBLIC_BASE_URL", "http://localhost:18081")
	t.Setenv("FAP_ADMIN_TOKEN", "fap-admin-token")

	cfg := LoadFromEnv()
	if cfg.AdminToken != "dev-admin-token" {
		t.Fatalf("expected admin token to load")
	}
	if len(cfg.ProviderTargets) != 2 {
		t.Fatalf("expected two provider targets, got %d", len(cfg.ProviderTargets))
	}
	if cfg.ProviderTargets[0].Name != "eu_1" || cfg.ProviderTargets[0].PublicBaseURL != "http://localhost:18082" || cfg.ProviderTargets[0].InternalBaseURL != "http://audistro-provider_eu_1:8080" || cfg.ProviderTargets[0].DataPathMount != "/mnt/providers/eu_1" {
		t.Fatalf("unexpected first provider target: %#v", cfg.ProviderTargets[0])
	}
	if cfg.ProviderTargets[1].Name != "eu_2" || cfg.ProviderTargets[1].PublicBaseURL != "http://localhost:18083" || cfg.ProviderTargets[1].InternalBaseURL != "http://audistro-provider_eu_2:8080" || cfg.ProviderTargets[1].DataPathMount != "/mnt/providers/eu_2" {
		t.Fatalf("unexpected second provider target: %#v", cfg.ProviderTargets[1])
	}
	if cfg.StoragePath != "/var/lib/audistro-catalog" {
		t.Fatalf("unexpected storage path %q", cfg.StoragePath)
	}
	if cfg.FAPInternalBaseURL != "http://audistro-fap:8080" {
		t.Fatalf("unexpected fap internal base url %q", cfg.FAPInternalBaseURL)
	}
	if cfg.FAPPublicBaseURL != "http://localhost:18081" {
		t.Fatalf("unexpected fap public base url %q", cfg.FAPPublicBaseURL)
	}
	if cfg.FAPAdminToken != "fap-admin-token" {
		t.Fatalf("unexpected fap admin token %q", cfg.FAPAdminToken)
	}
	if cfg.AdminUploadMaxBodyBytes <= 0 {
		t.Fatalf("expected positive admin upload max body bytes")
	}
}

func TestLoadFromEnvFallsBackToSingleProviderTarget(t *testing.T) {
	t.Setenv("CATALOG_PROVIDER_TARGETS", "")
	t.Setenv("CATALOG_PROVIDER_PUBLIC_BASE_URL", "http://localhost:18082")
	t.Setenv("CATALOG_PROVIDER_INTERNAL_BASE_URL", "http://audistro-provider_eu_1:8080")

	cfg := LoadFromEnv()
	if len(cfg.ProviderTargets) != 1 {
		t.Fatalf("expected single fallback provider target, got %d", len(cfg.ProviderTargets))
	}
	if cfg.ProviderTargets[0].PublicBaseURL != "http://localhost:18082" || cfg.ProviderTargets[0].InternalBaseURL != "http://audistro-provider_eu_1:8080" {
		t.Fatalf("unexpected fallback provider target: %#v", cfg.ProviderTargets[0])
	}
}

func TestLoadFromEnvOpenAPIValidationEnabledByDefault(t *testing.T) {
	t.Setenv("CATALOG_DISABLE_OPENAPI_VALIDATION", "")

	cfg := LoadFromEnv()
	if cfg.DisableOpenAPIValidation {
		t.Fatalf("expected openapi validation enabled by default")
	}
}

func TestLoadFromEnvAllowsDisablingOpenAPIValidation(t *testing.T) {
	t.Setenv("CATALOG_DISABLE_OPENAPI_VALIDATION", "true")

	cfg := LoadFromEnv()
	if !cfg.DisableOpenAPIValidation {
		t.Fatalf("expected openapi validation to be disabled")
	}
}

func TestValidateRequiresAdminTokenInDev(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "dev")
	t.Setenv("CATALOG_ENV", "")
	t.Setenv("CATALOG_ADMIN_TOKEN", "")

	cfg := LoadFromEnv()
	if err := cfg.Validate(); err == nil || err.Error() != "CATALOG_ADMIN_TOKEN is required when CATALOG_ENV=dev" {
		t.Fatalf("expected dev admin token validation error, got %v", err)
	}
}

func TestValidateAllowsProdWithoutAdminToken(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "prod")
	t.Setenv("CATALOG_ENV", "")
	t.Setenv("CATALOG_ADMIN_TOKEN", "")

	cfg := LoadFromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected prod config to validate without admin token, got %v", err)
	}
	if cfg.AdminEnabled() {
		t.Fatalf("expected admin disabled in prod")
	}
}

func TestReadOnlyDisablesAdminEndpointsEvenInDev(t *testing.T) {
	t.Setenv("AUDICATALOG_ENV", "dev")
	t.Setenv("CATALOG_ENV", "")
	t.Setenv("CATALOG_READ_ONLY", "true")
	t.Setenv("CATALOG_ADMIN_TOKEN", "dev-admin-token")

	cfg := LoadFromEnv()
	if !cfg.ReadOnly {
		t.Fatalf("expected read-only mode enabled")
	}
	if cfg.AdminEnabled() {
		t.Fatalf("expected admin disabled when read-only")
	}
}
