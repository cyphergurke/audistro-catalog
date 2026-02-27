package handlers

import (
	"errors"
	"net/http"

	assetsvc "audistro-catalog/internal/service/assets"
)

type ListArtistAssetsResponse struct {
	Assets []assetsvc.Asset `json:"assets"`
}

func ListArtistAssetsHandler(service *assetsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "asset service not configured")
			return
		}

		handle := r.PathValue("handle")
		assets, err := service.ListAssetsByArtistHandle(r.Context(), handle)
		if err != nil {
			if errors.Is(err, assetsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "artist_not_found", "artist not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to list assets")
			return
		}

		writeJSON(w, http.StatusOK, ListArtistAssetsResponse{Assets: assets})
	}
}
