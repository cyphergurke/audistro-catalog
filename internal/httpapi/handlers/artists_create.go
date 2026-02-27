package handlers

import (
	"errors"
	"net/http"

	artistsvc "github.com/cyphergurke/audistro-catalog/internal/service/artists"
)

type CreateArtistRequest struct {
	PubKeyHex   string `json:"pubkey_hex"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
}

type ArtistResponse struct {
	Artist artistsvc.Artist `json:"artist"`
}

func CreateArtistHandler(service *artistsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "artist service not configured")
			return
		}

		var req CreateArtistRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := validatePubKeyHex(req.PubKeyHex); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateHandle(req.Handle); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateDisplayName(req.DisplayName); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateBio(req.Bio); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateOptionalHTTPURL(req.AvatarURL, "invalid_avatar_url", "avatar_url", 2048); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}

		artist, err := service.CreateArtist(r.Context(), artistsvc.CreateArtistInput{
			PubKeyHex:   req.PubKeyHex,
			Handle:      req.Handle,
			DisplayName: req.DisplayName,
			Bio:         req.Bio,
			AvatarURL:   req.AvatarURL,
		})
		if err != nil {
			if errors.Is(err, artistsvc.ErrConflict) {
				writeError(w, r, http.StatusConflict, "artist_conflict", "artist with given handle or pubkey already exists")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to create artist")
			return
		}

		writeJSON(w, http.StatusCreated, ArtistResponse{Artist: artist})
	}
}
