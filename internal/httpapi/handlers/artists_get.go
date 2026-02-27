package handlers

import (
	"errors"
	"net/http"

	artistsvc "github.com/cyphergurke/audistro-catalog/internal/service/artists"
)

func GetArtistByHandleHandler(service *artistsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "artist service not configured")
			return
		}

		handle := r.PathValue("handle")
		artist, err := service.GetArtistByHandle(r.Context(), handle)
		if err != nil {
			if errors.Is(err, artistsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "artist_not_found", "artist not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to load artist")
			return
		}

		writeJSON(w, http.StatusOK, ArtistResponse{Artist: artist})
	}
}
