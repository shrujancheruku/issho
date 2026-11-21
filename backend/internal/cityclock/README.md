# cityclock

The one city-clock helper ("One city clock" in `docs/spec/architecture.md`). Every time on a city's clock, such as "09:00 on the day", quiet hours or a round's end, is computed here from the city's time zone, so daylight saving and the date line are handled in one place. It is pure: it never reads the system clock, and callers pass instants from an injected `clock.Clock`.

```go
city, err := cityclock.New("Asia/Ho_Chi_Minh", cityclock.DayParts{
	Morning:   cityclock.Span{From: "06:00", To: "12:00"},
	Afternoon: cityclock.Span{From: "12:00", To: "17:00"},
	Evening:   cityclock.Span{From: "17:00", To: "22:00"},
})
city.Date(now)                           // "2026-11-21", the city's date
city.At("2026-11-21", "09:00")           // that wall time as an instant
city.PartOfDay(now)                      // Morning, Afternoon, Evening or None
city.InWindow(now, "22:00", "07:00")     // quiet hours, across midnight
```

- `New` takes an IANA zone and the city's clock hours of morning, afternoon and evening (a city setting, `docs/spec/city-settings.md`); it fails on an unknown zone or a span that isn't `HH:MM`.
- Spans and windows include their start and exclude their end. A window whose end is before its start crosses midnight; equal ends are empty. An instant in none of the three parts is `None`.
- `At` puts a time the clocks skip (spring forward) after the gap, and takes the first of a time they repeat (fall back).

Tests (`cityclock_test.go`) cover a zone ahead of UTC (Asia/Ho_Chi_Minh), one behind (America/Los_Angeles, with its 2026 spring-forward and fall-back) and Australia/Sydney's summer time.
