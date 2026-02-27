package jobs

import (
	"context"
	"log"
	"time"

	"audistro-catalog/internal/store/repo"
)

func CleanupExpiredProviderAssetsOnce(ctx context.Context, providersRepo repo.ProviderRegistryRepository, now time.Time) (int64, time.Duration, error) {
	started := time.Now()
	deleted, err := providersRepo.DeleteExpiredProviderAssets(ctx, now.Unix())
	return deleted, time.Since(started), err
}

func StartCleanupExpiredProviderAssets(ctx context.Context, logger *log.Logger, providersRepo repo.ProviderRegistryRepository, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted, duration, err := CleanupExpiredProviderAssetsOnce(ctx, providersRepo, time.Now())
				if err != nil {
					logger.Printf("provider_assets_cleanup error=%v", err)
					continue
				}
				logger.Printf("provider_assets_cleanup cleanup_deleted_count=%d cleanup_duration_ms=%d", deleted, duration.Milliseconds())
			}
		}
	}()
}
