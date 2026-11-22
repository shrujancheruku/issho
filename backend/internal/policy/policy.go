// Package policy holds every number in the spec's rules as a constant: none is
// in the database, and none varies by city (docs/spec/architecture.md, "The
// seams"). Each constant's comment links the spec document that sets it, and
// `make policy-lint` fails on one that doesn't, or whose link points at a file
// that isn't there.
//
// A number changes only by a PR that edits that document and the constant
// together. Other backend packages import these names and never write the
// number again; the same lint flags a literal that repeats one.
package policy

import "time"

// A group is 3 to 6 people (docs/spec/matching.md). Matching never forms a
// group outside this range, and an outing is set once MinGroupSize say yes.
const (
	MinGroupSize = 3 // docs/spec/matching.md
	MaxGroupSize = 6 // docs/spec/matching.md
)

// Matching prefers groups of 4 or 5 over the ends of the range
// (docs/spec/matching.md, "Sizes").
const (
	PreferredGroupSizeLow  = 4 // docs/spec/matching.md
	PreferredGroupSizeHigh = 5 // docs/spec/matching.md
)

// InvitationWindow is how long an invitation stays open: a fixed 48 hours
// counted from delivery, so quiet hours never eat into it (docs/spec/outings.md).
const InvitationWindow = 48 * time.Hour

// YesesToSet is how many yeses set an outing: it is set as soon as that many
// invited people say yes, and with fewer when the invitations' 48 hours end it
// is not set (docs/spec/outings.md, "Invited").
const YesesToSet = 3 // docs/spec/outings.md

// RoundLength is a plan's round: a fixed 7 days, the same for everyone signed
// up (docs/spec/matching.md).
const RoundLength = 7 * 24 * time.Hour

// CheckInOpensBeforeStart is how long before an outing's start check-in opens;
// it stays open until the plan's stated end (docs/spec/outings.md).
const CheckInOpensBeforeStart = 30 * time.Minute

// FeedbackWindow is how long feedback stays open after the plan's stated end,
// and when a provisional no-show becomes final (docs/spec/outings.md).
const FeedbackWindow = 7 * 24 * time.Hour

// MinAge is the age a person must be on the day they give their date of birth:
// Issho is for adults only (docs/spec/people.md, "Sign-in and the account").
const MinAge = 18 // docs/spec/people.md

// A profile's name is 1 to 40 characters (docs/spec/people.md, "The profile").
const (
	NameMinLength = 1  // docs/spec/people.md
	NameMaxLength = 40 // docs/spec/people.md
)

// MinTickLeadDays is how far ahead a tick must be: a person ticks parts of the
// day on real dates from 3 days ahead, counted in calendar days on the city's
// clock (docs/spec/matching.md, "Rounds"). It is also the least notice an
// outing's date gets after its invitations go out.
const MinTickLeadDays = 3 // docs/spec/matching.md
