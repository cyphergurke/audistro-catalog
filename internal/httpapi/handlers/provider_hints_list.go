package handlers

import (
	"errors"
	"net/http"

	assetsvc "audistro-catalog/internal/service/assets"
)

type ListProviderHintsResponse struct {
	Hints []assetsvc.ProviderHint `json:"hints"`
}

func ListProviderHintsHandler(service *assetsvc.Service) http.HandlerFunc {
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

		hints, err := service.ListProviderHints(r.Context(), assetID)
		if err != nil {
			if errors.Is(err, assetsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to list provider hints")
			return
		}

		writeJSON(w, http.StatusOK, ListProviderHintsResponse{Hints: hints})
	}
}
