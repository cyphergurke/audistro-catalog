package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHTTPAddr                    = ":8081"
	defaultEnv                         = "prod"
	defaultDBPath                      = "audicatalog.db"
	defaultMaxAnnounceTTLSeconds int64 = 1209600
	defaultMaxProvidersPerAsset  int64 = 200

	defaultNonceCacheTTLSeconds       int64 = 600
	defaultNonceCacheMaxEntries       int64 = 100000
	defaultCleanupIntervalSeconds     int64 = 600
	defaultProvidersQueryDefaultLimit int64 = 20
	defaultProvidersQueryMaxLimit     int64 = 100
	defaultProviderStaleThreshold     int64 = 86400
	defaultProviderRecent10MBonus     int64 = 20
	defaultProviderRecent1HBonus      int64 = 10
	defaultProviderOld24HPenalty      int64 = 20
	defaultProviderExpires1HPenalty   int64 = 30
	defaultProviderExpires24HPenalty  int64 = 10
	defaultProviderPriorityMultiplier int64 = 3
	defaultProviderPriorityMax        int64 = 30
	defaultPlaybackProviderLimit      int64 = 10
	defaultPlaybackProviderMaxLimit   int64 = 50
	defaultAPISchemaVersion           int64 = 1
	defaultHTTPMaxBodyBytes           int64 = 32768
	defaultAdminUploadMaxBodyBytes    int64 = 268435456
	defaultETagMaxAgeSeconds          int64 = 5
	defaultRateLimitAnnounceRPS             = 5.0
	defaultRateLimitAnnounceBurst     int64 = 10
	defaultRateLimitPlaybackRPS             = 20.0
	defaultRateLimitPlaybackBurst     int64 = 40
	defaultRateLimitCacheTTLSeconds   int64 = 600
	defaultStoragePath                      = "/var/lib/audistro-catalog"
	defaultProviderPublicBaseURL            = "http://localhost:18082"
	defaultProviderInternalBaseURL          = "http://audistro-provider_eu_1:8080"
	defaultProviderDataPathMount            = "/mnt/providers/eu_1"
	defaultFAPPublicBaseURL                 = "http://localhost:18081"
	defaultFAPInternalBaseURL               = "http://audistro-fap:8080"
	defaultWorkerPollIntervalSeconds  int64 = 2
	defaultWorkerStaleSeconds         int64 = 300
)

type ProviderTarget struct {
	Name            string
	PublicBaseURL   string
	InternalBaseURL string
	DataPathMount   string
}

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	HTTPAddr                        string
	LogLevel                        string
	PublicBaseURL                   string
	Env                             string
	AllowInsecureTransport          bool
	DBPath                          string
	MaxAnnounceTTLSeconds           int64
	MaxProvidersPerAsset            int64
	NonceCacheTTLSeconds            int64
	NonceCacheMaxEntries            int64
	CleanupIntervalSeconds          int64
	ProvidersQueryDefaultLimit      int64
	ProvidersQueryMaxLimit          int64
	ProviderStaleThresholdSeconds   int64
	ProviderScoreRecent10MBonus     int64
	ProviderScoreRecent1HBonus      int64
	ProviderScoreOld24HPenalty      int64
	ProviderScoreExpires1HPenalty   int64
	ProviderScoreExpires24HPenalty  int64
	ProviderScorePriorityMultiplier int64
	ProviderScorePriorityMax        int64
	DefaultKeyURITemplate           string
	PlaybackDefaultProviderLimit    int64
	PlaybackMaxProviderLimit        int64
	APISchemaVersion                int
	DisableOpenAPIValidation        bool
	ReadOnly                        bool
	HTTPMaxBodyBytes                int64
	AdminUploadMaxBodyBytes         int64
	ETagMaxAgeSeconds               int64
	RateLimitAnnounceRPS            float64
	RateLimitAnnounceBurst          int64
	RateLimitPlaybackRPS            float64
	RateLimitPlaybackBurst          int64
	RateLimitCacheTTLSeconds        int64
	AdminToken                      string
	StoragePath                     string
	ProviderPublicBaseURL           string
	ProviderInternalBaseURL         string
	ProviderTargets                 []ProviderTarget
	FAPPublicBaseURL                string
	FAPInternalBaseURL              string
	FAPAdminToken                   string
	WorkerPollIntervalSeconds       int64
	WorkerStaleSeconds              int64
}

func (c Config) AdminEnabled() bool {
	return c.Env == "dev" && !c.ReadOnly
}

func (c Config) Validate() error {
	if c.AdminEnabled() && strings.TrimSpace(c.AdminToken) == "" {
		return fmt.Errorf("CATALOG_ADMIN_TOKEN is required when CATALOG_ENV=dev")
	}
	return nil
}

// LoadFromEnv reads configuration from environment variables.
func LoadFromEnv() Config {
	httpAddr := os.Getenv("AUDICATALOG_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	env := os.Getenv("AUDICATALOG_ENV")
	if env == "" {
		env = os.Getenv("CATALOG_ENV")
	}
	if env == "" {
		env = defaultEnv
	}
	env = strings.ToLower(strings.TrimSpace(env))
	if env != "dev" && env != "prod" {
		env = defaultEnv
	}

	dbPath := os.Getenv("AUDICATALOG_DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	maxAnnounceTTL := parseInt64OrDefault(os.Getenv("CATALOG_MAX_ANNOUNCE_TTL_SECONDS"), defaultMaxAnnounceTTLSeconds)
	maxProvidersPerAsset := parseInt64OrDefault(os.Getenv("CATALOG_MAX_PROVIDERS_PER_ASSET"), defaultMaxProvidersPerAsset)
	nonceCacheTTL := parseInt64OrDefault(os.Getenv("CATALOG_NONCE_CACHE_TTL_SECONDS"), defaultNonceCacheTTLSeconds)
	nonceCacheMaxEntries := parseInt64OrDefault(os.Getenv("CATALOG_NONCE_CACHE_MAX_ENTRIES"), defaultNonceCacheMaxEntries)
	cleanupIntervalSeconds := parseInt64OrDefault(os.Getenv("CATALOG_CLEANUP_INTERVAL_SECONDS"), defaultCleanupIntervalSeconds)
	providersQueryDefaultLimit := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDERS_QUERY_DEFAULT_LIMIT"), defaultProvidersQueryDefaultLimit)
	providersQueryMaxLimit := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDERS_QUERY_MAX_LIMIT"), defaultProvidersQueryMaxLimit)
	providerStaleThreshold := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_STALE_THRESHOLD_SECONDS"), defaultProviderStaleThreshold)
	providerRecent10MBonus := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_RECENT_10M_BONUS"), defaultProviderRecent10MBonus)
	providerRecent1HBonus := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_RECENT_1H_BONUS"), defaultProviderRecent1HBonus)
	providerOld24HPenalty := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_OLD_24H_PENALTY"), defaultProviderOld24HPenalty)
	providerExpires1HPenalty := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_EXPIRES_1H_PENALTY"), defaultProviderExpires1HPenalty)
	providerExpires24HPenalty := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_EXPIRES_24H_PENALTY"), defaultProviderExpires24HPenalty)
	providerPriorityMultiplier := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_PRIORITY_MULTIPLIER"), defaultProviderPriorityMultiplier)
	providerPriorityMax := parseInt64OrDefault(os.Getenv("CATALOG_PROVIDER_SCORE_PRIORITY_MAX"), defaultProviderPriorityMax)
	playbackDefaultProviderLimit := parseInt64OrDefault(os.Getenv("CATALOG_PLAYBACK_DEFAULT_PROVIDER_LIMIT"), defaultPlaybackProviderLimit)
	playbackMaxProviderLimit := parseInt64OrDefault(os.Getenv("CATALOG_PLAYBACK_MAX_PROVIDER_LIMIT"), defaultPlaybackProviderMaxLimit)
	apiSchemaVersion := int(parseInt64OrDefault(os.Getenv("CATALOG_API_SCHEMA_VERSION"), defaultAPISchemaVersion))
	disableOpenAPIValidation := parseBoolOrDefault(os.Getenv("CATALOG_DISABLE_OPENAPI_VALIDATION"), false)
	readOnly := parseBoolOrDefault(os.Getenv("CATALOG_READ_ONLY"), false)
	httpMaxBodyBytes := parseInt64OrDefault(os.Getenv("CATALOG_HTTP_MAX_BODY_BYTES"), defaultHTTPMaxBodyBytes)
	adminUploadMaxBodyBytes := parseInt64OrDefault(os.Getenv("CATALOG_ADMIN_UPLOAD_MAX_BODY_BYTES"), defaultAdminUploadMaxBodyBytes)
	etagMaxAgeSeconds := parseInt64OrDefault(os.Getenv("CATALOG_ETAG_MAX_AGE_SECONDS"), defaultETagMaxAgeSeconds)
	rateLimitAnnounceRPS := parseFloatOrDefault(os.Getenv("CATALOG_RL_ANNOUNCE_RPS"), defaultRateLimitAnnounceRPS)
	rateLimitAnnounceBurst := parseInt64OrDefault(os.Getenv("CATALOG_RL_ANNOUNCE_BURST"), defaultRateLimitAnnounceBurst)
	rateLimitPlaybackRPS := parseFloatOrDefault(os.Getenv("CATALOG_RL_PLAYBACK_RPS"), defaultRateLimitPlaybackRPS)
	rateLimitPlaybackBurst := parseInt64OrDefault(os.Getenv("CATALOG_RL_PLAYBACK_BURST"), defaultRateLimitPlaybackBurst)
	rateLimitCacheTTLSeconds := parseInt64OrDefault(os.Getenv("CATALOG_RL_CACHE_TTL_SECONDS"), defaultRateLimitCacheTTLSeconds)
	allowInsecureTransport := parseBoolOrDefault(os.Getenv("CATALOG_ALLOW_INSECURE_TRANSPORT"), false)
	storagePath := strings.TrimSpace(os.Getenv("CATALOG_STORAGE_PATH"))
	if storagePath == "" {
		storagePath = defaultStoragePath
	}
	providerPublicBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("CATALOG_PROVIDER_PUBLIC_BASE_URL")), "/")
	if providerPublicBaseURL == "" {
		providerPublicBaseURL = defaultProviderPublicBaseURL
	}
	providerInternalBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("CATALOG_PROVIDER_INTERNAL_BASE_URL")), "/")
	if providerInternalBaseURL == "" {
		providerInternalBaseURL = defaultProviderInternalBaseURL
	}
	providerTargets := parseProviderTargets(strings.TrimSpace(os.Getenv("CATALOG_PROVIDER_TARGETS")))
	if len(providerTargets) == 0 {
		providerTargets = []ProviderTarget{{
			Name:            "primary",
			PublicBaseURL:   providerPublicBaseURL,
			InternalBaseURL: providerInternalBaseURL,
			DataPathMount:   defaultProviderDataPathMount,
		}}
	}
	fapPublicBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FAP_PUBLIC_BASE_URL")), "/")
	if fapPublicBaseURL == "" {
		fapPublicBaseURL = defaultFAPPublicBaseURL
	}
	fapInternalBaseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("CATALOG_FAP_INTERNAL_BASE_URL")), "/")
	if fapInternalBaseURL == "" {
		fapInternalBaseURL = defaultFAPInternalBaseURL
	}
	workerPollIntervalSeconds := parseInt64OrDefault(os.Getenv("CATALOG_WORKER_POLL_INTERVAL_SECONDS"), defaultWorkerPollIntervalSeconds)
	workerStaleSeconds := parseInt64OrDefault(os.Getenv("CATALOG_WORKER_STALE_SECONDS"), defaultWorkerStaleSeconds)

	return Config{
		HTTPAddr:                        httpAddr,
		LogLevel:                        os.Getenv("AUDICATALOG_LOG_LEVEL"),
		PublicBaseURL:                   os.Getenv("AUDICATALOG_PUBLIC_BASE_URL"),
		Env:                             env,
		AllowInsecureTransport:          allowInsecureTransport,
		DBPath:                          dbPath,
		MaxAnnounceTTLSeconds:           maxAnnounceTTL,
		MaxProvidersPerAsset:            maxProvidersPerAsset,
		NonceCacheTTLSeconds:            nonceCacheTTL,
		NonceCacheMaxEntries:            nonceCacheMaxEntries,
		CleanupIntervalSeconds:          cleanupIntervalSeconds,
		ProvidersQueryDefaultLimit:      providersQueryDefaultLimit,
		ProvidersQueryMaxLimit:          providersQueryMaxLimit,
		ProviderStaleThresholdSeconds:   providerStaleThreshold,
		ProviderScoreRecent10MBonus:     providerRecent10MBonus,
		ProviderScoreRecent1HBonus:      providerRecent1HBonus,
		ProviderScoreOld24HPenalty:      providerOld24HPenalty,
		ProviderScoreExpires1HPenalty:   providerExpires1HPenalty,
		ProviderScoreExpires24HPenalty:  providerExpires24HPenalty,
		ProviderScorePriorityMultiplier: providerPriorityMultiplier,
		ProviderScorePriorityMax:        providerPriorityMax,
		DefaultKeyURITemplate:           os.Getenv("CATALOG_DEFAULT_KEY_URI_TEMPLATE"),
		PlaybackDefaultProviderLimit:    playbackDefaultProviderLimit,
		PlaybackMaxProviderLimit:        playbackMaxProviderLimit,
		APISchemaVersion:                apiSchemaVersion,
		DisableOpenAPIValidation:        disableOpenAPIValidation,
		ReadOnly:                        readOnly,
		HTTPMaxBodyBytes:                httpMaxBodyBytes,
		AdminUploadMaxBodyBytes:         adminUploadMaxBodyBytes,
		ETagMaxAgeSeconds:               etagMaxAgeSeconds,
		RateLimitAnnounceRPS:            rateLimitAnnounceRPS,
		RateLimitAnnounceBurst:          rateLimitAnnounceBurst,
		RateLimitPlaybackRPS:            rateLimitPlaybackRPS,
		RateLimitPlaybackBurst:          rateLimitPlaybackBurst,
		RateLimitCacheTTLSeconds:        rateLimitCacheTTLSeconds,
		AdminToken:                      strings.TrimSpace(os.Getenv("CATALOG_ADMIN_TOKEN")),
		StoragePath:                     storagePath,
		ProviderPublicBaseURL:           providerPublicBaseURL,
		ProviderInternalBaseURL:         providerInternalBaseURL,
		ProviderTargets:                 providerTargets,
		FAPPublicBaseURL:                fapPublicBaseURL,
		FAPInternalBaseURL:              fapInternalBaseURL,
		FAPAdminToken:                   strings.TrimSpace(os.Getenv("FAP_ADMIN_TOKEN")),
		WorkerPollIntervalSeconds:       workerPollIntervalSeconds,
		WorkerStaleSeconds:              workerStaleSeconds,
	}
}

func (c Config) IsInsecureTransportAllowed() bool {
	return c.Env == "dev" || c.AllowInsecureTransport
}

func parseInt64OrDefault(value string, fallback int64) int64 {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseFloatOrDefault(value string, fallback float64) float64 {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseBoolOrDefault(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseProviderTargets(raw string) []ProviderTarget {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	targets := make([]ProviderTarget, 0)
	for _, item := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(item), "|")
		if len(parts) != 4 {
			continue
		}

		target := ProviderTarget{
			Name:            strings.TrimSpace(parts[0]),
			PublicBaseURL:   normalizeURL(parts[1]),
			InternalBaseURL: normalizeURL(parts[2]),
			DataPathMount:   strings.TrimRight(strings.TrimSpace(parts[3]), "/"),
		}
		if target.Name == "" || target.PublicBaseURL == "" || target.InternalBaseURL == "" || target.DataPathMount == "" {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func normalizeURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.TrimRight(parsed.String(), "/")
}

func (c Config) PrimaryProviderTarget() (ProviderTarget, error) {
	if len(c.ProviderTargets) == 0 {
		return ProviderTarget{}, fmt.Errorf("no provider targets configured")
	}
	return c.ProviderTargets[0], nil
}
