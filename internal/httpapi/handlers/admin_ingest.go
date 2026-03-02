package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	ingestsvc "audistro-catalog/internal/service/ingest"
)

type adminUploadResponse struct {
	AssetID string `json:"asset_id"`
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
}

type adminIngestJobResponse struct {
	JobID   string `json:"job_id"`
	AssetID string `json:"asset_id"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

func AdminUploadAssetHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.IngestService == nil {
			writeError(w, r, http.StatusServiceUnavailable, "ingest_unavailable", "ingest service unavailable")
			return
		}
		if err := requireDevAdmin(deps, r); err != nil {
			writeAdminError(w, r, err)
			return
		}
		maxBodyBytes := deps.AdminUploadMaxBodyBytes
		if maxBodyBytes <= 0 {
			maxBodyBytes = 256 << 20
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_multipart", "invalid multipart form")
			return
		}

		priceMSat, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("price_msat")), 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_price_msat", "price_msat must be an integer")
			return
		}
		file, _, err := r.FormFile("audio")
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "audio_required", "audio file is required")
			return
		}
		defer file.Close()

		job, err := deps.IngestService.QueueUpload(r.Context(), ingestsvc.QueueUploadInput{
			ArtistID:  r.FormValue("artist_id"),
			PayeeID:   r.FormValue("payee_id"),
			Title:     r.FormValue("title"),
			PriceMSat: priceMSat,
			AssetID:   r.FormValue("asset_id"),
			Source:    file,
		})
		if err != nil {
			writeIngestServiceError(w, r, err)
			return
		}

		writeJSON(w, http.StatusAccepted, adminUploadResponse{
			AssetID: job.AssetID,
			JobID:   job.JobID,
			Status:  job.Status,
		})
	}
}

func GetIngestJobHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.IngestService == nil {
			writeError(w, r, http.StatusServiceUnavailable, "ingest_unavailable", "ingest service unavailable")
			return
		}
		if err := requireDevAdmin(deps, r); err != nil {
			writeAdminError(w, r, err)
			return
		}
		job, err := deps.IngestService.GetJob(r.Context(), r.PathValue("jobId"))
		if err != nil {
			writeIngestServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, adminIngestJobResponse{
			JobID:   job.JobID,
			AssetID: job.AssetID,
			Status:  job.Status,
			Error:   job.Error,
		})
	}
}

func requireDevAdmin(deps Dependencies, r *http.Request) error {
	if !deps.AdminEnabled {
		return ingestsvc.ErrAdminDisabled
	}
	if strings.TrimSpace(deps.AdminToken) == "" {
		return ingestsvc.ErrAdminDisabled
	}
	if subtleHeaderValue(r.Header.Get("X-Admin-Token")) != deps.AdminToken {
		return ingestsvc.ErrUnauthorized
	}
	return nil
}

func subtleHeaderValue(value string) string {
	return strings.TrimSpace(value)
}

func writeAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ingestsvc.ErrAdminDisabled):
		writeError(w, r, http.StatusForbidden, "admin_disabled", "admin upload is only available in dev")
	case errors.Is(err, ingestsvc.ErrUnauthorized):
		writeError(w, r, http.StatusUnauthorized, "invalid_admin_token", "invalid admin token")
	default:
		writeError(w, r, http.StatusInternalServerError, "admin_error", "admin request failed")
	}
}

func writeIngestServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ingestsvc.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, ingestsvc.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, ingestsvc.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", err.Error())
	default:
		writeError(w, r, http.StatusInternalServerError, "ingest_failed", fmt.Sprintf("ingest request failed: %v", err))
	}
}
