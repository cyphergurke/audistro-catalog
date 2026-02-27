package handlers

import (
	"errors"
	"net/http"

	payeessvc "github.com/cyphergurke/audistro-catalog/internal/service/payees"
)

type CreatePayeeRequest struct {
	ArtistHandle     string `json:"artist_handle"`
	FAPPublicBaseURL string `json:"fap_public_base_url"`
	FAPPayeeID       string `json:"fap_payee_id"`
}

type PayeeResponse struct {
	Payee payeessvc.Payee `json:"payee"`
}

func CreatePayeeHandler(service *payeessvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "payee service not configured")
			return
		}

		var req CreatePayeeRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := validateHandle(req.ArtistHandle); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_artist_handle", "artist_handle must match ^[a-z0-9][a-z0-9_]{2,31}$")
			return
		}
		if err := validateRequiredHTTPURL(req.FAPPublicBaseURL, "invalid_fap_public_base_url", "fap_public_base_url"); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if req.FAPPayeeID == "" || len(req.FAPPayeeID) > 128 {
			writeError(w, r, http.StatusBadRequest, "invalid_fap_payee_id", "fap_payee_id must be 1..128 characters")
			return
		}

		payee, err := service.CreatePayee(r.Context(), payeessvc.CreatePayeeInput{
			ArtistHandle:     req.ArtistHandle,
			FAPPublicBaseURL: req.FAPPublicBaseURL,
			FAPPayeeID:       req.FAPPayeeID,
		})
		if err != nil {
			if errors.Is(err, payeessvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "artist_not_found", "artist_handle was not found")
				return
			}
			if errors.Is(err, payeessvc.ErrConflict) {
				writeError(w, r, http.StatusConflict, "payee_conflict", "payee already exists")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to create payee")
			return
		}

		writeJSON(w, http.StatusCreated, PayeeResponse{Payee: payee})
	}
}
