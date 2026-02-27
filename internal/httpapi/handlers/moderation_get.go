package handlers

import (
	"errors"
	"net/http"

	reportsvc "github.com/cyphergurke/audistro-catalog/internal/service/reports"
)

type GetModerationResponse struct {
	Moderation reportsvc.ModerationState `json:"moderation"`
}

func GetModerationHandler(service *reportsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "reports service not configured")
			return
		}

		targetType := r.PathValue("targetType")
		targetID := r.PathValue("targetId")

		moderation, err := service.GetModeration(r.Context(), targetType, targetID)
		if err != nil {
			var validationErr reportsvc.ValidationError
			if errors.As(err, &validationErr) {
				writeError(w, r, http.StatusBadRequest, validationErr.Code, validationErr.Message)
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to load moderation")
			return
		}

		writeJSON(w, http.StatusOK, GetModerationResponse{Moderation: moderation})
	}
}
