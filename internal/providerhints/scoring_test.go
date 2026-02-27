package providerhints

import "testing"

func TestComputeHintScoreHighPriorityRecentFarExpiryHigher(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	now := int64(1_700_000_000)

	high := ComputeHintScore(now, 10, now-60, now+7*24*60*60, cfg)
	low := ComputeHintScore(now, 1, now-2*24*60*60, now+2*60*60, cfg)
	if high <= low {
		t.Fatalf("expected high score > low score, got %d <= %d", high, low)
	}
}

func TestComputeHintScoreNearExpiryPenalty(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	now := int64(1_700_000_000)

	near := ComputeHintScore(now, 5, now-60, now+30*60, cfg)
	far := ComputeHintScore(now, 5, now-60, now+72*60*60, cfg)
	if near >= far {
		t.Fatalf("expected near-expiry score < far-expiry score, got %d >= %d", near, far)
	}
}

func TestComputeHintScoreOldLastSeenPenalty(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	now := int64(1_700_000_000)

	old := ComputeHintScore(now, 5, now-25*60*60, now+72*60*60, cfg)
	recent := ComputeHintScore(now, 5, now-5*60, now+72*60*60, cfg)
	if old >= recent {
		t.Fatalf("expected old score < recent score, got %d >= %d", old, recent)
	}
}

func TestComputeHintScoreClamp(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	now := int64(1_700_000_000)

	high := ComputeHintScore(now, 1000, now, now+365*24*60*60, cfg)
	if high != 100 {
		t.Fatalf("expected clamp to 100, got %d", high)
	}
	low := ComputeHintScore(now, 0, now-100*24*60*60, now+1, cfg)
	if low != 0 {
		t.Fatalf("expected clamp to 0, got %d", low)
	}
}
