package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cyphergurke/audistro-catalog/internal/noncecache"
	"github.com/cyphergurke/audistro-catalog/internal/validate"
	providersvc "github.com/cyphergurke/audistro-catalog/internal/service/providers"
)

type AnnounceProviderRequest struct {
	AssetID          string `json:"asset_id"`
	Transport        string `json:"transport"`
	BaseURL          string `json:"base_url"`
	Priority         int64  `json:"priority"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
	ExpiresAt        int64  `json:"expires_at"`
	Nonce            string `json:"nonce"`
	Signature        string `json:"signature"`
}

func AnnounceProviderHandler(service *providersvc.Service, cache *noncecache.Cache, nonceCacheTTLSeconds int64, insecureAllowed bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "provider service not configured")
			return
		}
		if cache == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "nonce cache not configured")
			return
		}

		providerID := r.PathValue("providerId")
		if providerID == "" {
			writeError(w, r, http.StatusBadRequest, "invalid_provider_id", "providerId path is required")
			return
		}

		var req AnnounceProviderRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			if isRequestBodyTooLarge(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_, _ = w.Write([]byte(`{"error":"body_too_large"}`))
				return
			}
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if len(req.BaseURL) > 512 {
			writeError(w, r, http.StatusBadRequest, "invalid_base_url", "base_url must be <= 512 chars")
			return
		}
		if len(req.Nonce) > 128 {
			writeError(w, r, http.StatusBadRequest, "invalid_nonce", "nonce must be <= 128 chars")
			return
		}
		if len(strings.TrimSpace(req.Signature)) != 128 {
			writeError(w, r, http.StatusBadRequest, "invalid_signature", "signature must be 128 hex chars")
			return
		}

		ttl := time.Duration(nonceCacheTTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = 10 * time.Minute
		}
		nonceKey := providerID + "|" + req.AssetID + "|" + strings.ToLower(strings.TrimSpace(req.Nonce))
		if cache.AddIfNotExists(nonceKey, ttl, time.Now()) {
			writeJSON(w, http.StatusConflict, struct {
				Error string `json:"error"`
			}{Error: "nonce_replay"})
			return
		}

		// validate announce base_url/transport
		normalizedBase, normalizedTransport, _, verr := validate.NormalizeAndValidateBaseURL(req.BaseURL, req.Transport, insecureAllowed)
		if verr != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid announce base_url/transport")
			return
		}

		err := service.Announce(r.Context(), providersvc.AnnounceInput{
			ProviderID:       providerID,
			AssetID:          req.AssetID,
			Transport:        normalizedTransport,
			BaseURL:          normalizedBase,
			Priority:         req.Priority,
			ExpiresInSeconds: req.ExpiresInSeconds,
			ExpiresAt:        req.ExpiresAt,
			Nonce:            req.Nonce,
			Signature:        req.Signature,
		})
		if err != nil {
			if errors.Is(err, providersvc.ErrInvalidInput) {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid announcement request")
				return
			}
			if errors.Is(err, providersvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "not_found", "provider or asset not found")
				return
			}
			if errors.Is(err, providersvc.ErrCapacityExceeded) {
				writeError(w, r, http.StatusTooManyRequests, "asset_provider_limit_reached", "asset provider limit reached")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to process announcement")
			return
		}

		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
		}{Status: "ok"})
	}
}
