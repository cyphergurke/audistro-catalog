package handlers

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var handlePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{2,31}$`)
var assetIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,80}$`)
var adminIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

type validationError struct {
	Code    string
	Message string
}

func decodeStrictJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("body is required")
		}
		return err
	}
	if dec.More() {
		return fmt.Errorf("body must contain a single JSON object")
	}
	return nil
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func validatePubKeyHex(value string) *validationError {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return &validationError{Code: "invalid_pubkey_hex", Message: "pubkey_hex is required"}
	}
	if len(trimmed) != 64 && len(trimmed) != 66 {
		return &validationError{Code: "invalid_pubkey_hex", Message: "pubkey_hex must have length 64 or 66"}
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return &validationError{Code: "invalid_pubkey_hex", Message: "pubkey_hex must be valid hexadecimal"}
	}
	return nil
}

func validateHandle(value string) *validationError {
	if !handlePattern.MatchString(value) {
		return &validationError{Code: "invalid_handle", Message: "handle must match ^[a-z0-9][a-z0-9_]{2,31}$"}
	}
	return nil
}

func validateDisplayName(value string) *validationError {
	if value == "" {
		return &validationError{Code: "invalid_display_name", Message: "display_name is required"}
	}
	if len(value) > 80 {
		return &validationError{Code: "invalid_display_name", Message: "display_name must be at most 80 characters"}
	}
	return nil
}

func validateBio(value string) *validationError {
	if len(value) > 2000 {
		return &validationError{Code: "invalid_bio", Message: "bio must be at most 2000 characters"}
	}
	return nil
}

func validateOptionalHTTPURL(value string, code string, field string, maxLen int) *validationError {
	if value == "" {
		return nil
	}
	if len(value) > maxLen {
		return &validationError{Code: code, Message: field + " is too long"}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return &validationError{Code: code, Message: field + " must be a valid URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &validationError{Code: code, Message: field + " must use http or https"}
	}
	return nil
}

func validateRequiredHTTPURL(value string, code string, field string) *validationError {
	if strings.TrimSpace(value) == "" {
		return &validationError{Code: code, Message: field + " is required"}
	}
	return validateOptionalHTTPURL(value, code, field, 2048)
}

func validateRequiredHTTPURLMax(value string, code string, field string, maxLen int) *validationError {
	if strings.TrimSpace(value) == "" {
		return &validationError{Code: code, Message: field + " is required"}
	}
	return validateOptionalHTTPURL(value, code, field, maxLen)
}

func validateOptionalAssetID(value string) *validationError {
	if value == "" {
		return nil
	}
	if !assetIDPattern.MatchString(value) {
		return &validationError{Code: "invalid_asset_id", Message: "asset_id must match ^[a-zA-Z0-9_-]{3,80}$"}
	}
	return nil
}

func validateTitle(value string) *validationError {
	if value == "" {
		return &validationError{Code: "invalid_title", Message: "title is required"}
	}
	if len(value) > 200 {
		return &validationError{Code: "invalid_title", Message: "title must be at most 200 characters"}
	}
	return nil
}

func validateDurationMS(value int64) *validationError {
	const maxDurationMS = 24 * 60 * 60 * 1000
	if value <= 0 || value >= maxDurationMS {
		return &validationError{Code: "invalid_duration_ms", Message: "duration_ms must be > 0 and < 86400000"}
	}
	return nil
}

func validateContentID(value string) *validationError {
	if strings.TrimSpace(value) == "" {
		return &validationError{Code: "invalid_content_id", Message: "content_id is required"}
	}
	if len(value) > 200 {
		return &validationError{Code: "invalid_content_id", Message: "content_id must be at most 200 characters"}
	}
	return nil
}

func validatePriority(priority int64) *validationError {
	if priority < 0 || priority > 100 {
		return &validationError{Code: "invalid_priority", Message: "priority must be in range 0..100"}
	}
	return nil
}

func validateTransport(transport string) *validationError {
	switch transport {
	case "http", "https", "ipfs", "torrent":
		return nil
	default:
		return &validationError{Code: "invalid_transport", Message: "transport must be one of http, https, ipfs, torrent"}
	}
}

func validateProviderHintBaseURL(transport string, baseURL string) *validationError {
	if strings.TrimSpace(baseURL) == "" {
		return &validationError{Code: "invalid_base_url", Message: "base_url is required"}
	}
	if len(baseURL) > 2048 {
		return &validationError{Code: "invalid_base_url", Message: "base_url is too long"}
	}

	switch transport {
	case "http", "https":
		return validateOptionalHTTPURL(baseURL, "invalid_base_url", "base_url", 2048)
	case "ipfs":
		if strings.HasPrefix(baseURL, "ipfs://") {
			return nil
		}
		return validateOptionalHTTPURL(baseURL, "invalid_base_url", "base_url", 2048)
	case "torrent":
		if strings.HasPrefix(baseURL, "magnet:?") {
			return nil
		}
		return &validationError{Code: "invalid_base_url", Message: "torrent base_url must be a magnet URI"}
	default:
		return &validationError{Code: "invalid_transport", Message: "unsupported transport"}
	}
}
