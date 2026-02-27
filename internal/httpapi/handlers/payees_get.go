package handlers

import (
	"errors"
	"net/http"

	payeessvc "audistro-catalog/internal/service/payees"
)

func GetPayeeHandler(service *payeessvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "payee service not configured")
			return
		}

		payeeID := r.PathValue("payeeId")
		payee, err := service.GetPayee(r.Context(), payeeID)
		if err != nil {
			if errors.Is(err, payeessvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "payee_not_found", "payee not found")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to load payee")
			return
		}

		writeJSON(w, http.StatusOK, PayeeResponse{Payee: payee})
	}
}
