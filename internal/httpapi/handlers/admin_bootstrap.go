package handlers

import (
	"errors"
	"net/http"
	"strings"

	bootstrapsvc "audistro-catalog/internal/service/bootstrap"
)

type BootstrapArtistRequest struct {
	ArtistID    string                `json:"artist_id"`
	Handle      string                `json:"handle"`
	DisplayName string                `json:"display_name"`
	PubKeyHex   string                `json:"pubkey_hex"`
	Payee       BootstrapPayeeRequest `json:"payee"`
}

type BootstrapPayeeRequest struct {
	PayeeID          string `json:"payee_id"`
	FAPPublicBaseURL string `json:"fap_public_base_url"`
	FAPPayeeID       string `json:"fap_payee_id"`
}

type BootstrapArtistResponse struct {
	ArtistID   string `json:"artist_id"`
	PayeeID    string `json:"payee_id"`
	Handle     string `json:"handle"`
	FAPPayeeID string `json:"fap_payee_id"`
}

func BootstrapArtistHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.BootstrapService == nil {
			writeError(w, r, http.StatusServiceUnavailable, "service_unavailable", "bootstrap service unavailable")
			return
		}
		if err := requireDevAdmin(deps, r); err != nil {
			writeAdminError(w, r, err)
			return
		}

		var req BootstrapArtistRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if strings.TrimSpace(req.ArtistID) != "" && !adminIDPattern.MatchString(strings.TrimSpace(req.ArtistID)) {
			writeError(w, r, http.StatusBadRequest, "invalid_artist_id", "artist_id is invalid")
			return
		}
		if err := validateHandle(strings.TrimSpace(req.Handle)); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateDisplayName(strings.TrimSpace(req.DisplayName)); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if strings.TrimSpace(req.PubKeyHex) != "" {
			if err := validatePubKeyHex(strings.TrimSpace(req.PubKeyHex)); err != nil {
				writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
				return
			}
		}
		if strings.TrimSpace(req.Payee.PayeeID) != "" && !adminIDPattern.MatchString(strings.TrimSpace(req.Payee.PayeeID)) {
			writeError(w, r, http.StatusBadRequest, "invalid_payee_id", "payee_id is invalid")
			return
		}
		if err := validateRequiredHTTPURL(req.Payee.FAPPublicBaseURL, "invalid_fap_public_base_url", "fap_public_base_url"); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if strings.TrimSpace(req.Payee.FAPPayeeID) == "" || len(strings.TrimSpace(req.Payee.FAPPayeeID)) > 128 {
			writeError(w, r, http.StatusBadRequest, "invalid_fap_payee_id", "fap_payee_id must be 1..128 characters")
			return
		}

		result, err := deps.BootstrapService.BootstrapArtist(r.Context(), bootstrapsvc.BootstrapArtistInput{
			ArtistID:    strings.TrimSpace(req.ArtistID),
			Handle:      strings.TrimSpace(req.Handle),
			DisplayName: strings.TrimSpace(req.DisplayName),
			PubKeyHex:   strings.TrimSpace(req.PubKeyHex),
			Payee: bootstrapsvc.BootstrapPayeeInput{
				PayeeID:          strings.TrimSpace(req.Payee.PayeeID),
				FAPPublicBaseURL: strings.TrimSpace(req.Payee.FAPPublicBaseURL),
				FAPPayeeID:       strings.TrimSpace(req.Payee.FAPPayeeID),
			},
		})
		if err != nil {
			switch {
			case errors.Is(err, bootstrapsvc.ErrConflict):
				writeError(w, r, http.StatusConflict, "bootstrap_conflict", err.Error())
			case errors.Is(err, bootstrapsvc.ErrNotFound):
				writeError(w, r, http.StatusNotFound, "bootstrap_not_found", err.Error())
			default:
				writeError(w, r, http.StatusInternalServerError, "bootstrap_failed", "failed to bootstrap artist")
			}
			return
		}

		writeJSON(w, http.StatusOK, BootstrapArtistResponse(result))
	}
}
