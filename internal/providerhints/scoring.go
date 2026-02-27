package providerhints

type Config struct {
	StaleThresholdSeconds int64
	Recent10MBonus        int
	Recent1HBonus         int
	Old24HPenalty         int
	Expires1HPenalty      int
	Expires24HPenalty     int
	PriorityMultiplier    int
	PriorityMax           int
}

func DefaultConfig() Config {
	return Config{
		StaleThresholdSeconds: 86400,
		Recent10MBonus:        20,
		Recent1HBonus:         10,
		Old24HPenalty:         20,
		Expires1HPenalty:      30,
		Expires24HPenalty:     10,
		PriorityMultiplier:    3,
		PriorityMax:           30,
	}
}

func ComputeHintScore(now int64, priority int, lastSeenAt int64, expiresAt int64, cfg Config) int {
	score := 50

	priorityScore := priority * cfg.PriorityMultiplier
	if priorityScore > cfg.PriorityMax {
		priorityScore = cfg.PriorityMax
	}
	if priorityScore < 0 {
		priorityScore = 0
	}
	score += priorityScore

	age := now - lastSeenAt
	switch {
	case age <= 10*60:
		score += cfg.Recent10MBonus
	case age <= 60*60:
		score += cfg.Recent1HBonus
	case age > 24*60*60:
		score -= cfg.Old24HPenalty
	}

	expiresIn := expiresAt - now
	switch {
	case expiresIn <= 60*60:
		score -= cfg.Expires1HPenalty
	case expiresIn <= 24*60*60:
		score -= cfg.Expires24HPenalty
	}

	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
