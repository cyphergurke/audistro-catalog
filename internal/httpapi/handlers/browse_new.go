package handlers

import (
	"net/http"
	"sort"

	artistsvc "github.com/cyphergurke/audistro-catalog/internal/service/artists"
	assetsvc "github.com/cyphergurke/audistro-catalog/internal/service/assets"
)

type BrowseNewResponse struct {
	Assets []assetsvc.Asset `json:"assets"`
}

func BrowseNewHandler(artistsService *artistsvc.Service, assetsService *assetsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if artistsService == nil || assetsService == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "browse services not configured")
			return
		}

		artists, err := artistsService.ListArtists(r.Context())
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to browse new")
			return
		}

		assets := make([]assetsvc.Asset, 0)
		for _, artist := range artists {
			if artist.Moderation.State == "delist" {
				continue
			}
			artistAssets, listErr := assetsService.ListAssetsByArtistHandle(r.Context(), artist.Handle)
			if listErr != nil {
				writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to browse new")
				return
			}
			assets = append(assets, artistAssets...)
		}

		sort.Slice(assets, func(i, j int) bool {
			return assets[i].CreatedAt > assets[j].CreatedAt
		})

		writeJSON(w, http.StatusOK, BrowseNewResponse{Assets: assets})
	}
}
