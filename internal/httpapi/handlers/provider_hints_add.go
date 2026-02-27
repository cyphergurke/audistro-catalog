package handlers

import (
	"errors"
	"net/http"

	assetsvc "github.com/cyphergurke/audistro-catalog/internal/service/assets"
)

type AddProviderHintRequest struct {
	Transport string `json:"transport"`
	BaseURL   string `json:"base_url"`
	Priority  int64  `json:"priority"`
}

type AddProviderHintResponse struct {
	Hint assetsvc.ProviderHint `json:"hint"`
}

func AddProviderHintHandler(service *assetsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "asset service not configured")
			return
		}

		assetID := r.PathValue("assetId")
		if assetID == "" {
			writeError(w, r, http.StatusBadRequest, "invalid_asset_id", "assetId is required")
			return
		}

		var req AddProviderHintRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := validateTransport(req.Transport); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateProviderHintBaseURL(req.Transport, req.BaseURL); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validatePriority(req.Priority); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}

		hint, err := service.AddProviderHint(r.Context(), assetID, assetsvc.CreateProviderHintInput{
			Transport: req.Transport,
			BaseURL:   req.BaseURL,
			Priority:  req.Priority,
		})
		if err != nil {
			if errors.Is(err, assetsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
				return
			}
			if errors.Is(err, assetsvc.ErrConflict) {
				writeError(w, r, http.StatusConflict, "provider_hint_conflict", "provider hint conflict")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to add provider hint")
			return
		}

		writeJSON(w, http.StatusCreated, AddProviderHintResponse{Hint: hint})
	}
}
