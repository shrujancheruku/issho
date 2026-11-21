package policy

import "time"

const (
	MinGroupSize = 3 // docs/spec/matching.md
	MaxGroupSize = 6 // docs/spec/matching.md
)

const InvitationWindow = 48 * time.Hour // docs/spec/matching.md

const RoundLength = 7 * 24 * time.Hour // docs/spec/matching.md

const FeedbackWindow = 7 * 24 * time.Hour // docs/spec/matching.md
