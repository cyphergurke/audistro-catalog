package reports

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"audistro-catalog/internal/model"
	"audistro-catalog/internal/store/repo"
)

var ErrValidation = errors.New("validation error")

type ValidationError struct {
	Code    string
	Message string
}

func (e ValidationError) Error() string {
	return e.Code + ": " + e.Message
}

type CreateReportInput struct {
	ReporterSubject string
	TargetType      string
	TargetID        string
	ReportType      string
	Evidence        string
}

type ModerationState struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	State      string `json:"state"`
	ReasonCode string `json:"reason_code"`
	UpdatedAt  int64  `json:"updated_at"`
}

type Service struct {
	reportsRepo      repo.ReportsRepository
	moderationRepo   repo.ModerationRepository
	artistsRepo      repo.ArtistsRepository
	verificationRepo repo.VerificationRepository
}

func NewService(
	reportsRepo repo.ReportsRepository,
	moderationRepo repo.ModerationRepository,
	artistsRepo repo.ArtistsRepository,
	verificationRepo repo.VerificationRepository,
) *Service {
	return &Service{
		reportsRepo:      reportsRepo,
		moderationRepo:   moderationRepo,
		artistsRepo:      artistsRepo,
		verificationRepo: verificationRepo,
	}
}

func (s *Service) CreateReport(ctx context.Context, input CreateReportInput) (string, ModerationState, error) {
	if err := validateCreateReportInput(input); err != nil {
		return "", ModerationState{}, fmt.Errorf("validate report input: %w", err)
	}

	reportID := generateReportID()
	_, err := s.reportsRepo.InsertReport(ctx, repo.InsertReportParams{
		ReportID:        reportID,
		ReporterSubject: input.ReporterSubject,
		TargetType:      input.TargetType,
		TargetID:        input.TargetID,
		ReportType:      input.ReportType,
		Evidence:        input.Evidence,
	})
	if err != nil {
		return "", ModerationState{}, fmt.Errorf("insert report: %w", err)
	}

	moderation, err := s.evaluateAndUpsert(ctx, input.TargetType, input.TargetID)
	if err != nil {
		return "", ModerationState{}, fmt.Errorf("evaluate report decision: %w", err)
	}
	return reportID, moderation, nil
}

func (s *Service) GetModeration(ctx context.Context, targetType string, targetID string) (ModerationState, error) {
	if targetType == "" || targetID == "" {
		return ModerationState{}, ValidationError{Code: "invalid_target", Message: "targetType and targetId are required"}
	}
	if !isTargetType(targetType) {
		return ModerationState{}, ValidationError{Code: "invalid_target_type", Message: "target_type is invalid"}
	}

	state, exists, err := s.moderationRepo.GetState(ctx, targetType, targetID)
	if err != nil {
		return ModerationState{}, fmt.Errorf("get moderation state: %w", err)
	}
	if !exists {
		return allowState(targetType, targetID), nil
	}

	return ModerationState{
		TargetType: state.TargetType,
		TargetID:   state.TargetID,
		State:      state.State,
		ReasonCode: state.ReasonCode,
		UpdatedAt:  state.UpdatedAt,
	}, nil
}

func (s *Service) evaluateAndUpsert(ctx context.Context, targetType string, targetID string) (ModerationState, error) {
	sinceUnix := time.Now().Add(-7 * 24 * time.Hour).Unix()

	decision := allowState(targetType, targetID)
	switch targetType {
	case "artist":
		impCount, err := s.reportsRepo.CountReports(ctx, targetType, targetID, "impersonation", sinceUnix)
		if err != nil {
			return ModerationState{}, fmt.Errorf("count artist impersonation reports: %w", err)
		}
		if impCount >= 3 {
			decision.State = "delist"
			decision.ReasonCode = "impersonation_threshold"
			verified, vErr := s.isArtistVerified(ctx, targetID)
			if vErr != nil {
				return ModerationState{}, vErr
			}
			if verified {
				decision.State = "warn"
				decision.ReasonCode = "impersonation_threshold_verified"
			}
		}
	case "asset":
		scamCount, err := s.reportsRepo.CountReports(ctx, targetType, targetID, "scam", sinceUnix)
		if err != nil {
			return ModerationState{}, fmt.Errorf("count asset scam reports: %w", err)
		}
		malwareCount, err := s.reportsRepo.CountReports(ctx, targetType, targetID, "malware", sinceUnix)
		if err != nil {
			return ModerationState{}, fmt.Errorf("count asset malware reports: %w", err)
		}
		spamCount, err := s.reportsRepo.CountReports(ctx, targetType, targetID, "spam", sinceUnix)
		if err != nil {
			return ModerationState{}, fmt.Errorf("count asset spam reports: %w", err)
		}

		if scamCount >= 2 {
			decision.State = "quarantine"
			decision.ReasonCode = "scam_threshold"
		} else if malwareCount >= 2 {
			decision.State = "quarantine"
			decision.ReasonCode = "malware_threshold"
		} else if spamCount >= 3 {
			decision.State = "warn"
			decision.ReasonCode = "spam_threshold"
		}
	case "url":
		malwareCount, err := s.reportsRepo.CountReports(ctx, targetType, targetID, "malware", sinceUnix)
		if err != nil {
			return ModerationState{}, fmt.Errorf("count url malware reports: %w", err)
		}
		if malwareCount >= 1 {
			decision.State = "quarantine"
			decision.ReasonCode = "malware_url"
		}
	}

	upserted, err := s.moderationRepo.UpsertState(ctx, repo.UpsertModerationStateParams{
		TargetType: decision.TargetType,
		TargetID:   decision.TargetID,
		State:      decision.State,
		ReasonCode: decision.ReasonCode,
	})
	if err != nil {
		return ModerationState{}, fmt.Errorf("upsert moderation state: %w", err)
	}

	decision.UpdatedAt = upserted.UpdatedAt
	return decision, nil
}

func (s *Service) isArtistVerified(ctx context.Context, artistID string) (bool, error) {
	artist, err := s.artistsRepo.GetArtistByID(ctx, model.ArtistID(artistID))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get artist for verification lookup: %w", err)
	}

	verification, err := s.verificationRepo.GetByPubKeyHex(ctx, artist.PubKeyHex)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get verification state: %w", err)
	}

	return verification.Badge == "verified" || verification.Badge == "highly_verified", nil
}

func validateCreateReportInput(input CreateReportInput) error {
	if len(input.ReporterSubject) > 200 {
		return ValidationError{Code: "invalid_reporter_subject", Message: "reporter_subject must be <= 200 chars"}
	}
	if !isTargetType(input.TargetType) {
		return ValidationError{Code: "invalid_target_type", Message: "target_type must be one of artist, asset, url"}
	}
	if strings.TrimSpace(input.TargetID) == "" || len(input.TargetID) > 512 {
		return ValidationError{Code: "invalid_target_id", Message: "target_id must be non-empty and <= 512 chars"}
	}
	if !isReportType(input.ReportType) {
		return ValidationError{Code: "invalid_report_type", Message: "report_type is invalid"}
	}
	if len(input.Evidence) > 4000 {
		return ValidationError{Code: "invalid_evidence", Message: "evidence must be <= 4000 chars"}
	}
	if input.TargetType == "url" {
		parsed, err := url.Parse(input.TargetID)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return ValidationError{Code: "invalid_target_id", Message: "url target_id must be a valid http(s) URL"}
		}
	}
	return nil
}

func isTargetType(targetType string) bool {
	switch targetType {
	case "artist", "asset", "url":
		return true
	default:
		return false
	}
}

func isReportType(reportType string) bool {
	switch reportType {
	case "impersonation", "scam", "spam", "malware", "copyright":
		return true
	default:
		return false
	}
}

func allowState(targetType string, targetID string) ModerationState {
	return ModerationState{
		TargetType: targetType,
		TargetID:   targetID,
		State:      "allow",
		ReasonCode: "none",
	}
}

func generateReportID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return fmt.Sprintf("rpt_%x", buf)
	}
	return fmt.Sprintf("rpt_%d", time.Now().UnixNano())
}
