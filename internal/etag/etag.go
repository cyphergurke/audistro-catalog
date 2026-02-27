package etag

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func ComputeETag(parts ...string) string {
	source := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(source))
	return "\"" + hex.EncodeToString(hash[:]) + "\""
}

func MatchesIfNoneMatch(headerValue string, tag string) bool {
	if strings.TrimSpace(headerValue) == "" {
		return false
	}
	if strings.TrimSpace(headerValue) == "*" {
		return true
	}
	for _, candidate := range strings.Split(headerValue, ",") {
		if strings.TrimSpace(candidate) == tag {
			return true
		}
	}
	return false
}
