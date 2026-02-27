package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/cyphergurke/audistro-catalog/internal/model"
)

const (
	BadgeUnverified     = "unverified"
	BadgeVerified       = "verified"
	BadgeHighlyVerified = "highly_verified"
	BadgeFlagged        = "flagged"
)

type ScoreResult struct {
	Badge      string
	Score      int64
	InputsHash string
}

// ComputeState calculates badge/score from proof statuses and moderation snapshot.
func ComputeState(proofs []model.ArtistProof, moderation *model.ModerationState) ScoreResult {
	hasDomainTXT := false
	hasWellKnown := false

	sortedProofs := append([]model.ArtistProof(nil), proofs...)
	sort.Slice(sortedProofs, func(i, j int) bool {
		if sortedProofs[i].ProofType == sortedProofs[j].ProofType {
			return sortedProofs[i].ProofID < sortedProofs[j].ProofID
		}
		return sortedProofs[i].ProofType < sortedProofs[j].ProofType
	})

	parts := make([]string, 0, len(sortedProofs)+1)
	for _, proof := range sortedProofs {
		parts = append(parts, proof.ProofID+"|"+string(proof.ProofType)+"|"+proof.ProofValue+"|"+string(proof.Status))
		if proof.Status != model.ProofStatusVerified {
			continue
		}
		switch proof.ProofType {
		case model.ProofTypeDomainTXT:
			hasDomainTXT = true
		case model.ProofTypeWellKnown:
			hasWellKnown = true
		}
	}

	badge := BadgeUnverified
	score := int64(0)
	if hasDomainTXT || hasWellKnown {
		badge = BadgeVerified
		score = 80
	}
	if hasDomainTXT && hasWellKnown {
		badge = BadgeHighlyVerified
		score = 120
	}

	moderationPart := "moderation:none"
	if moderation != nil {
		moderationPart = "moderation:" + moderation.State + "|" + moderation.ReasonCode
		if (moderation.State == "delist" || moderation.State == "quarantine") && strings.Contains(strings.ToLower(moderation.ReasonCode), "impersonation") {
			badge = BadgeFlagged
			score = 0
		}
	}
	parts = append(parts, moderationPart)

	h := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return ScoreResult{
		Badge:      badge,
		Score:      score,
		InputsHash: hex.EncodeToString(h[:]),
	}
}
