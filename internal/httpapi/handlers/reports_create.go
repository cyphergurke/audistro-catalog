package handlers

import (
	"errors"
	"net/http"

	reportsvc "audistro-catalog/internal/service/reports"
)

type CreateReportRequest struct {
	ReporterSubject string `json:"reporter_subject"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	ReportType      string `json:"report_type"`
	Evidence        string `json:"evidence"`
}

type CreateReportResponse struct {
	ReportID   string                    `json:"report_id"`
	Moderation reportsvc.ModerationState `json:"moderation"`
}

func CreateReportHandler(service *reportsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "reports service not configured")
			return
		}

		var req CreateReportRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		reportID, moderation, err := service.CreateReport(r.Context(), reportsvc.CreateReportInput{
			ReporterSubject: req.ReporterSubject,
			TargetType:      req.TargetType,
			TargetID:        req.TargetID,
			ReportType:      req.ReportType,
			Evidence:        req.Evidence,
		})
		if err != nil {
			var validationErr reportsvc.ValidationError
			if errors.As(err, &validationErr) {
				writeError(w, r, http.StatusBadRequest, validationErr.Code, validationErr.Message)
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to create report")
			return
		}

		writeJSON(w, http.StatusCreated, CreateReportResponse{ReportID: reportID, Moderation: moderation})
	}
}
