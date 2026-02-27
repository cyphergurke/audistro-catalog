package handlers

import (
	"errors"
	"net/http"

	payeessvc "github.com/cyphergurke/audistro-catalog/internal/service/payees"
)

type PayeesListResponse struct {
	Payees []payeessvc.Payee `json:"payees"`
}

func ListArtistPayeesHandler(service *payeessvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "payee service not configured")
			return
		}

		handle := r.PathValue("handle")
		payees, err := service.ListPayeesByArtistHandle(r.Context(), handle)
		if err != nil {
			if errors.Is(err, payeessvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "artist_not_found", "artist not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to list payees")
			return
		}

		writeJSON(w, http.StatusOK, PayeesListResponse{Payees: payees})
	}
}
