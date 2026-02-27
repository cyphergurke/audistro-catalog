package handlers

import (
	"errors"
	"net/http"

	assetsvc "github.com/cyphergurke/audistro-catalog/internal/service/assets"
)

type GetAssetResponse struct {
	Asset  assetsvc.Asset         `json:"asset"`
	Artist assetsvc.ArtistSummary `json:"artist"`
}

func GetAssetHandler(service *assetsvc.Service) http.HandlerFunc {
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

		value, err := service.GetAsset(r.Context(), assetID)
		if err != nil {
			if errors.Is(err, assetsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "asset_not_found", "asset not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to load asset")
			return
		}

		writeJSON(w, http.StatusOK, GetAssetResponse{Asset: value.Asset, Artist: value.Artist})
	}
}
