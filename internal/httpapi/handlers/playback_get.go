package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/etag"
	"github.com/cyphergurke/audistro-catalog/internal/providerhints"
	assetsvc "github.com/cyphergurke/audistro-catalog/internal/service/assets"
	payeessvc "github.com/cyphergurke/audistro-catalog/internal/service/payees"
	providersvc "github.com/cyphergurke/audistro-catalog/internal/service/providers"
	"github.com/cyphergurke/audistro-catalog/internal/validate"
)

func GetPlaybackHandler(
	assetsService *assetsvc.Service,
	hintsService *providerhints.Service,
	payeesService *payeessvc.Service,
	defaultKeyURITemplate string,
	defaultLimit int64,
	maxLimit int64,
	apiVersion string,
	schemaVersion int,
	etagMaxAgeSeconds int64,
	insecureAllowed bool,
) http.HandlerFunc {
	if apiVersion == "" {
		apiVersion = "v1"
	}
	if schemaVersion <= 0 {
		schemaVersion = 1
	}
	if etagMaxAgeSeconds <= 0 {
		etagMaxAgeSeconds = 5
	}
	_ = defaultKeyURITemplate
	return func(w http.ResponseWriter, r *http.Request) {
		if assetsService == nil || hintsService == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "playback service not configured")
			return
		}

		assetID := r.PathValue("assetId")
		if !playbackAssetIDPattern.MatchString(assetID) {
			writeError(w, r, http.StatusBadRequest, "invalid_asset_id", "assetId path is invalid")
			return
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

		limit := clampProviderLimitForPlayback(parseProvidersLimit(r.URL.Query().Get("limit")), defaultLimit, maxLimit)
		regionRaw := strings.TrimSpace(r.URL.Query().Get("region"))
		var region *string
		if regionRaw != "" {
			region = &regionRaw
		}

		now := time.Now().Unix()
		hints, err := hintsService.ListProvidersForAsset(r.Context(), assetID, region, limit, now)
		if err != nil {
			if errors.Is(err, providersvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
				return
			}
			if errors.Is(err, providersvc.ErrInvalidInput) {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid request")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to list provider hints")
			return
		}

		maxHintsUpdatedAt := int64(0)
		providers := make([]AssetProviderPayload, 0, len(hints))
		for _, hint := range hints {
			if hint.UpdatedAt > maxHintsUpdatedAt {
				maxHintsUpdatedAt = hint.UpdatedAt
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
			"playback",
			assetID,
			fmt.Sprintf("%d", assetWithArtist.Asset.UpdatedAt),
			fmt.Sprintf("%d", maxHintsUpdatedAt),
			fmt.Sprintf("%d", schemaVersion),
		)
		w.Header().Set("ETag", entityTag)
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", etagMaxAgeSeconds))
		if etag.MatchesIfNoneMatch(r.Header.Get("If-None-Match"), entityTag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		playbackAsset := AssetPlayback{
			AssetID:    assetWithArtist.Asset.AssetID,
			Title:      assetWithArtist.Asset.Title,
			DurationMS: assetWithArtist.Asset.DurationMS,
			HLS: PlaybackHLS{
				MasterPath: assetWithArtist.Asset.HLSMasterURL,
				Encryption: "fap",
			},
		}

		payeeID := strings.TrimSpace(assetWithArtist.Asset.PayeeID)
		if payeeID == "" || payeesService == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "key_uri_template_unavailable"})
			return
		}
		payee, perr := payeesService.GetPayee(r.Context(), payeeID)
		if perr != nil {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "key_uri_template_unavailable"})
			return
		}
		fapBase, _, _, verr := validate.NormalizeAndValidateBaseURL(payee.FAPPublicBaseURL, "", insecureAllowed)
		if verr != nil {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "key_uri_template_unavailable"})
			return
		}
		playbackAsset.HLS.KeyURITemplate = fapBase + "/hls/{asset_id}/key"

		purchase := assetWithArtist.Asset.Purchase
		resourceID := purchase.ResourceID
		if resourceID == "" {
			resourceID = "hls:key:" + assetWithArtist.Asset.AssetID
		}
		challengeURL := purchase.ChallengeURL
		if challengeURL == "" {
			challengeURL = fapBase + "/v1/fap/challenge"
		}
		tokenURL := purchase.TokenURL
		if tokenURL == "" {
			tokenURL = fapBase + "/v1/fap/token"
		}
		priceMSat := purchase.PriceMSat
		if priceMSat <= 0 {
			priceMSat = assetWithArtist.Asset.PriceMSat
		}
		playbackAsset.Pay = &PlaybackPay{
			ResourceID:   resourceID,
			ChallengeURL: challengeURL,
			TokenURL:     tokenURL,
			FAPURL:       fapBase,
			PayeeID:      payee.PayeeID,
			FAPPayeeID:   payee.FAPPayeeID,
			PriceMSat:    priceMSat,
		}

		writeJSON(w, http.StatusOK, PlaybackResponse{
			APIVersion:    apiVersion,
			SchemaVersion: schemaVersion,
			Now:           now,
			Asset:         playbackAsset,
			Providers:     providers,
		})
	}
}

func clampProviderLimitForPlayback(limit int, defaultLimit int64, maxLimit int64) int {
	if defaultLimit <= 0 {
		defaultLimit = 10
	}
	if maxLimit <= 0 {
		maxLimit = 50
	}
	if defaultLimit > maxLimit {
		defaultLimit = maxLimit
	}
	if limit <= 0 {
		return int(defaultLimit)
	}
	if int64(limit) > maxLimit {
		return int(maxLimit)
	}
	return limit
}
