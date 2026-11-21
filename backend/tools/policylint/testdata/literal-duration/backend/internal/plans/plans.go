package plans

import "time"

func expiry(t time.Time) time.Time { return t.Add(48 * time.Hour) }

func roundEnd(t time.Time) time.Time {
	return t.Add(time.Duration(7) * 24 * time.Hour)
}
