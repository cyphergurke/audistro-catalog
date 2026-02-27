package handlers

import (
	"net/http"

	artistsvc "audistro-catalog/internal/service/artists"
)

type BrowseArtistsResponse struct {
	Artists []artistsvc.Artist `json:"artists"`
}

func BrowseArtistsHandler(service *artistsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "artist service not configured")
			return
		}

		artists, err := service.ListArtists(r.Context())
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to browse artists")
			return
		}

		filtered := make([]artistsvc.Artist, 0, len(artists))
		for _, artist := range artists {
			if artist.Moderation.State == "delist" {
				continue
			}
			filtered = append(filtered, artist)
		}

		writeJSON(w, http.StatusOK, BrowseArtistsResponse{Artists: filtered})
	}
}
