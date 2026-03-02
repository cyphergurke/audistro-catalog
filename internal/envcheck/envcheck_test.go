package envcheck

import "testing"

func TestValidateSkipsWhenModeUnset(t *testing.T) {
	t.Setenv("AUDISTRO_ENV", "")

	if err := Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateFailsWhenProdEnvMissing(t *testing.T) {
	t.Setenv("AUDISTRO_ENV", "prod")
	t.Setenv("AUDICATALOG_DB_PATH", "/var/lib/audistro-catalog/audistro-catalog.db")
	t.Setenv("CATALOG_PROVIDER_TARGETS", "eu_1|https://provider-eu-1.example.com|http://audistro-provider_eu_1:8080|/mnt/providers/eu_1")
	t.Setenv("FAP_ADMIN_TOKEN", "")

	if err := Validate(); err == nil || err.Error() != "envcheck: missing required env: FAP_ADMIN_TOKEN" {
		t.Fatalf("expected missing fap admin token error, got %v", err)
	}
}

func TestValidateAcceptsProdEnv(t *testing.T) {
	t.Setenv("AUDISTRO_ENV", "prod")
	t.Setenv("AUDICATALOG_DB_PATH", "/var/lib/audistro-catalog/audistro-catalog.db")
	t.Setenv("CATALOG_PROVIDER_TARGETS", "eu_1|https://provider-eu-1.example.com|http://audistro-provider_eu_1:8080|/mnt/providers/eu_1")
	t.Setenv("FAP_ADMIN_TOKEN", "catalog-fap-admin")

	if err := Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
