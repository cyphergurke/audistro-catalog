package providerhints

import (
	"context"
	"sort"
	"strings"

	providersvc "github.com/cyphergurke/audistro-catalog/internal/service/providers"
)

type ServiceConfig struct {
	DefaultLimit int64
	MaxLimit     int64
	Score        Config
}

type Hint struct {
	ProviderID string
	Transport  string
	BaseURL    string
	Priority   int64
	ExpiresAt  int64
	LastSeenAt int64
	UpdatedAt  int64
	Region     string
	HintScore  int
	Stale      bool
}

type Service struct {
	providersService *providersvc.Service
	cfg              ServiceConfig
}

func NewService(providersService *providersvc.Service, cfg ServiceConfig) *Service {
	if cfg.DefaultLimit <= 0 {
		cfg.DefaultLimit = 20
	}
	if cfg.MaxLimit <= 0 {
		cfg.MaxLimit = 100
	}
	if cfg.DefaultLimit > cfg.MaxLimit {
		cfg.DefaultLimit = cfg.MaxLimit
	}
	if cfg.Score.StaleThresholdSeconds <= 0 {
		cfg.Score = DefaultConfig()
	}
	return &Service{
		providersService: providersService,
		cfg:              cfg,
	}
}

func (s *Service) ListProvidersForAsset(ctx context.Context, assetID string, region *string, limit int, now int64) ([]Hint, error) {
	if limit <= 0 {
		limit = int(s.cfg.DefaultLimit)
	}
	if int64(limit) > s.cfg.MaxLimit {
		limit = int(s.cfg.MaxLimit)
	}
	if limit < 1 {
		limit = 1
	}

	fetchLimit := int(s.cfg.MaxLimit)
	if fetchLimit < limit {
		fetchLimit = limit
	}

	providers, err := s.providersService.ListAssetProviders(ctx, assetID, fetchLimit)
	if err != nil {
		return nil, err
	}

	var preferredRegion string
	if region != nil {
		preferredRegion = strings.TrimSpace(*region)
	}

	hints := make([]Hint, 0, len(providers))
	for _, provider := range providers {
		hints = append(hints, Hint{
			ProviderID: provider.ProviderID,
			Transport:  provider.Transport,
			BaseURL:    provider.BaseURL,
			Priority:   provider.Priority,
			ExpiresAt:  provider.ExpiresAt,
			LastSeenAt: provider.LastSeenAt,
			UpdatedAt:  provider.UpdatedAt,
			Region:     provider.Region,
			HintScore:  ComputeHintScore(now, int(provider.Priority), provider.LastSeenAt, provider.ExpiresAt, s.cfg.Score),
			Stale:      (now - provider.LastSeenAt) > s.cfg.Score.StaleThresholdSeconds,
		})
	}

	sort.Slice(hints, func(i int, j int) bool {
		left := hints[i]
		right := hints[j]

		if preferredRegion != "" {
			leftMatch := strings.EqualFold(left.Region, preferredRegion)
			rightMatch := strings.EqualFold(right.Region, preferredRegion)
			if leftMatch != rightMatch {
				return leftMatch
			}
		}
		if left.HintScore != right.HintScore {
			return left.HintScore > right.HintScore
		}
		if left.LastSeenAt != right.LastSeenAt {
			return left.LastSeenAt > right.LastSeenAt
		}
		return left.ProviderID < right.ProviderID
	})

	if len(hints) > limit {
		hints = hints[:limit]
	}
	return hints, nil
}
