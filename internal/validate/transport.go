package validate

import (
    "errors"
    "net/url"
    "strings"
)

var (
    ErrInvalidURL         = errors.New("invalid_url")
    ErrInvalidScheme      = errors.New("invalid_scheme")
    ErrTransportMismatch  = errors.New("transport_mismatch")
    ErrInsecureNotAllowed = errors.New("insecure_not_allowed")
)

// NormalizeAndValidateBaseURL parses and normalizes a provider base URL and
// validates that the provided transport matches the URL scheme and that https
// is required when insecureAllowed is false.
func NormalizeAndValidateBaseURL(baseURL string, transport string, insecureAllowed bool) (string, string, string, error) {
    trimmed := strings.TrimSpace(baseURL)
    parsed, err := url.Parse(trimmed)
    if err != nil || parsed.Host == "" {
        return "", "", "", ErrInvalidURL
    }
    scheme := strings.ToLower(parsed.Scheme)
    if scheme != "http" && scheme != "https" {
        return "", "", "", ErrInvalidScheme
    }
    normTransport := strings.ToLower(strings.TrimSpace(transport))
    if normTransport == "" {
        normTransport = scheme
    }
    if normTransport != scheme {
        return "", "", "", ErrTransportMismatch
    }
    if !insecureAllowed && scheme != "https" {
        return "", "", "", ErrInsecureNotAllowed
    }

    // normalize base URL by trimming trailing '/'
    normalized := strings.TrimRight(parsed.String(), "/")
    return normalized, normTransport, scheme, nil
}
