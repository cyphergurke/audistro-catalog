package model

// ModerationState is a placeholder moderation record for later policy workflows.
type ModerationState struct {
	TargetType string
	TargetID   string
	State      string
	ReasonCode string
	UpdatedAt  int64
}
