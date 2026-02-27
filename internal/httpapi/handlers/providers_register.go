package handlers

import (
	"errors"
	"net/http"

	"github.com/cyphergurke/audistro-catalog/internal/validate"
	providersvc "github.com/cyphergurke/audistro-catalog/internal/service/providers"
)

type RegisterProviderRequest struct {
	ProviderID string `json:"provider_id"`
	PublicKey  string `json:"public_key"`
	Transport  string `json:"transport"`
	BaseURL    string `json:"base_url"`
	Region     string `json:"region"`
}

type ProviderResponse struct {
	Provider providersvc.Provider `json:"provider"`
}

func RegisterProviderHandler(service *providersvc.Service, insecureAllowed bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "provider service not configured")
			return
		}

		var req RegisterProviderRequest
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

		// validate and normalize base_url/transport
		normalizedBase, normalizedTransport, _, verr := validate.NormalizeAndValidateBaseURL(req.BaseURL, req.Transport, insecureAllowed)
		if verr != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid provider base_url/transport")
			return
		}

		provider, err := service.UpsertProvider(r.Context(), providersvc.UpsertProviderInput{
			ProviderID: req.ProviderID,
			PublicKey:  req.PublicKey,
			Transport:  normalizedTransport,
			BaseURL:    normalizedBase,
			Region:     req.Region,
		})
		if err != nil {
			if errors.Is(err, providersvc.ErrInvalidInput) {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid provider request")
				return
			}
			if errors.Is(err, providersvc.ErrConflict) {
				writeError(w, r, http.StatusConflict, "provider_key_mismatch", "provider_id already bound to different public_key")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to store provider")
			return
		}

		writeJSON(w, http.StatusOK, ProviderResponse{Provider: provider})
	}
}
