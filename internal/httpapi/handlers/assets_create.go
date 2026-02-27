package handlers

import (
	"errors"
	"net/http"

	assetsvc "audistro-catalog/internal/service/assets"
)

type CreateAssetProviderHintRequest struct {
	Transport string `json:"transport"`
	BaseURL   string `json:"base_url"`
	Priority  int64  `json:"priority"`
}

type CreateAssetRequest struct {
	AssetID       string                           `json:"asset_id"`
	ArtistHandle  string                           `json:"artist_handle"`
	PayeeID       string                           `json:"payee_id"`
	Title         string                           `json:"title"`
	DurationMS    int64                            `json:"duration_ms"`
	ContentID     string                           `json:"content_id"`
	HLSMasterURL  string                           `json:"hls_master_url"`
	PreviewHLSURL string                           `json:"preview_hls_url"`
	PriceMSat     int64                            `json:"price_msat"`
	ProviderHints []CreateAssetProviderHintRequest `json:"provider_hints"`
}

type CreateAssetResponse struct {
	Asset         assetsvc.Asset          `json:"asset"`
	ProviderHints []assetsvc.ProviderHint `json:"provider_hints"`
}

func CreateAssetHandler(service *assetsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeError(w, r, http.StatusInternalServerError, "service_unavailable", "asset service not configured")
			return
		}

		var req CreateAssetRequest
		if err := decodeStrictJSON(r, &req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		if err := validateOptionalAssetID(req.AssetID); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateHandle(req.ArtistHandle); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_artist_handle", "artist_handle must match ^[a-z0-9][a-z0-9_]{2,31}$")
			return
		}
		if req.PayeeID == "" {
			writeError(w, r, http.StatusBadRequest, "invalid_payee_id", "payee_id is required")
			return
		}
		if err := validateTitle(req.Title); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateDurationMS(req.DurationMS); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateContentID(req.ContentID); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateRequiredHTTPURLMax(req.HLSMasterURL, "invalid_hls_master_url", "hls_master_url", 2048); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if err := validateOptionalHTTPURL(req.PreviewHLSURL, "invalid_preview_hls_url", "preview_hls_url", 2048); err != nil {
			writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
			return
		}
		if req.PriceMSat < 0 {
			writeError(w, r, http.StatusBadRequest, "invalid_price_msat", "price_msat must be >= 0")
			return
		}
		if len(req.ProviderHints) > 10 {
			writeError(w, r, http.StatusBadRequest, "invalid_provider_hints", "provider_hints must contain at most 10 entries")
			return
		}

		hintsInput := make([]assetsvc.CreateProviderHintInput, 0, len(req.ProviderHints))
		for _, hint := range req.ProviderHints {
			if err := validateTransport(hint.Transport); err != nil {
				writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
				return
			}
			if err := validateProviderHintBaseURL(hint.Transport, hint.BaseURL); err != nil {
				writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
				return
			}
			if err := validatePriority(hint.Priority); err != nil {
				writeError(w, r, http.StatusBadRequest, err.Code, err.Message)
				return
			}
			hintsInput = append(hintsInput, assetsvc.CreateProviderHintInput{
				Transport: hint.Transport,
				BaseURL:   hint.BaseURL,
				Priority:  hint.Priority,
			})
		}

		asset, hints, err := service.CreateAsset(r.Context(), assetsvc.CreateAssetInput{
			AssetID:       req.AssetID,
			ArtistHandle:  req.ArtistHandle,
			PayeeID:       req.PayeeID,
			Title:         req.Title,
			DurationMS:    req.DurationMS,
			ContentID:     req.ContentID,
			HLSMasterURL:  req.HLSMasterURL,
			PreviewHLSURL: req.PreviewHLSURL,
			PriceMSat:     req.PriceMSat,
			ProviderHints: hintsInput,
		})
		if err != nil {
			if errors.Is(err, assetsvc.ErrPayeeMismatch) {
				writeError(w, r, http.StatusBadRequest, "payee_mismatch", "payee_id does not belong to artist_handle")
				return
			}
			if errors.Is(err, assetsvc.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, "not_found", "artist or payee was not found")
				return
			}
			if errors.Is(err, assetsvc.ErrConflict) {
				writeError(w, r, http.StatusConflict, "asset_conflict", "asset conflict")
				return
			}
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to create asset")
			return
		}

		writeJSON(w, http.StatusCreated, CreateAssetResponse{Asset: asset, ProviderHints: hints})
	}
}
