package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/etag"
	"github.com/cyphergurke/audistro-catalog/internal/providerhints"
	assetsvc "github.com/cyphergurke/audistro-catalog/internal/service/assets"
	providersvc "github.com/cyphergurke/audistro-catalog/internal/service/providers"
	"github.com/cyphergurke/audistro-catalog/internal/validate"
)

var playbackAssetIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

type ListAssetProvidersResponse struct {
	APIVersion    string                 `json:"api_version"`
	SchemaVersion int                    `json:"schema_version"`
	AssetID       string                 `json:"asset_id"`
	Now           int64                  `json:"now"`
	Providers     []AssetProviderPayload `json:"providers"`
}

type AssetProviderPayload struct {
	ProviderID string `json:"provider_id"`
	Transport  string `json:"transport"`
	BaseURL    string `json:"base_url"`
	Priority   int64  `json:"priority"`
	ExpiresAt  int64  `json:"expires_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	Region     string `json:"region,omitempty"`
	HintScore  int    `json:"hint_score"`
	Stale      bool   `json:"stale"`
}

func ListAssetProvidersHandler(assetsService *assetsvc.Service, hintsService *providerhints.Service, apiVersion string, schemaVersion int, etagMaxAgeSeconds int64) http.HandlerFunc {
	if apiVersion == "" {
		apiVersion = "v1"
	}
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	if etagMaxAgeSeconds <= 0 {
		etagMaxAgeSeconds = 5
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if assetsService == nil || hintsService == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "provider hints service not configured")
			return
		}

		assetID := r.PathValue("assetId")
		if !playbackAssetIDPattern.MatchString(assetID) {
			writeError(w, r, http.StatusBadRequest, "invalid_asset_id", "assetId path is invalid")
			return
		}

		limit := parseProvidersLimit(r.URL.Query().Get("limit"))
		regionRaw := strings.TrimSpace(r.URL.Query().Get("region"))
		var region *string
		if regionRaw != "" {
			region = &regionRaw
		}

		assetWithArtist, err := assetsService.GetAsset(r.Context(), assetID)
		if err != nil {
			if errors.Is(err, assetsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to load asset")
			return
		}

		now := time.Now().Unix()
		hints, err := hintsService.ListProvidersForAsset(r.Context(), assetID, region, limit, now)
		if err != nil {
			if errors.Is(err, providersvc.ErrInvalidInput) {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid asset id")
				return
			}
			if errors.Is(err, providersvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to list asset providers")
			return
		}

		maxHintsUpdatedAt := int64(0)
		maxHintsExpiresAt := int64(0)
		providers := make([]AssetProviderPayload, 0, len(hints))
		for _, hint := range hints {
			if hint.UpdatedAt > maxHintsUpdatedAt {
				maxHintsUpdatedAt = hint.UpdatedAt
			}
			if hint.ExpiresAt > maxHintsExpiresAt {
				maxHintsExpiresAt = hint.ExpiresAt
			}
			normalizedBase, _, scheme, verr := validate.NormalizeAndValidateBaseURL(hint.BaseURL, "", true)
			if verr != nil {
				continue
			}
			p := toProviderPayload(hint)
			p.BaseURL = normalizedBase
			p.Transport = scheme
			providers = append(providers, p)
		}

		entityTag := etag.ComputeETag(
			"providers",
			assetID,
			fmt.Sprintf("%d", assetWithArtist.Asset.UpdatedAt),
			fmt.Sprintf("%d", maxHintsUpdatedAt),
			fmt.Sprintf("%d", maxHintsExpiresAt),
			fmt.Sprintf("%d", schemaVersion),
		)
		w.Header().Set("ETag", entityTag)
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", etagMaxAgeSeconds))
		if etag.MatchesIfNoneMatch(r.Header.Get("If-None-Match"), entityTag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		writeJSON(w, http.StatusOK, ListAssetProvidersResponse{
			APIVersion:    apiVersion,
			SchemaVersion: schemaVersion,
			AssetID:       assetID,
			Now:           now,
			Providers:     providers,
		})
	}
}

func parseProvidersLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return parsed
}

func toProviderPayload(hint providerhints.Hint) AssetProviderPayload {
	return AssetProviderPayload{
		ProviderID: hint.ProviderID,
		Transport:  hint.Transport,
		BaseURL:    hint.BaseURL,
		Priority:   hint.Priority,
		ExpiresAt:  hint.ExpiresAt,
		LastSeenAt: hint.LastSeenAt,
		Region:     hint.Region,
		HintScore:  hint.HintScore,
		Stale:      hint.Stale,
	}
}
