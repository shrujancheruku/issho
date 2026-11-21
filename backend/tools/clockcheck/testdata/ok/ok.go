// Package ok is a fixture for clockcheck: it uses package time without
// reading the clock, and has look-alikes that are not package time.
package ok

import "time"

type fakeTime struct{}

func (fakeTime) Now() time.Time { return time.Unix(0, 0) }

func Uses(d time.Duration, t time.Time) time.Time {
	var time fakeTime // shadows the package: time.Now here is a method call
	_ = time.Now()
	return t.Add(d)
}

func Mentions() string { return "time.Now() in a string and a comment: time.Since" }
