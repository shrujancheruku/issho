package matching

import "testing"

// A city that sets no timing keeps the engine's original constants.
func TestDefaultTiming(t *testing.T) {
	want := Timing{
		LeadDays: 2, WindowDays: 14, SignupDays: 7, CooldownDays: 14,
		BlockStarts: map[Block]string{Morning: "10:00", Afternoon: "14:00", Evening: "18:30"},
	}
	cfg, err := ParseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Timing
	if got.LeadDays != want.LeadDays || got.WindowDays != want.WindowDays || got.SignupDays != want.SignupDays ||
		got.CooldownDays != want.CooldownDays || len(got.BlockStarts) != 3 {
		t.Fatalf("default timing = %+v, want %+v", got, want)
	}
	for b, s := range want.BlockStarts {
		if got.BlockStarts[b] != s {
			t.Errorf("block %s starts %q, want %q", b, got.BlockStarts[b], s)
		}
	}
}

func TestParseConfigTiming(t *testing.T) {
	// One block overridden; the others and the days keep their defaults.
	cfg, err := ParseConfig([]byte(`{"timing": {"block_starts": {"evening": "21:00"}, "signup_days": 5}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timing.BlockStarts[Evening] != "21:00" || cfg.Timing.BlockStarts[Morning] != "10:00" || cfg.Timing.SignupDays != 5 || cfg.Timing.WindowDays != 14 {
		t.Errorf("overlay = %+v", cfg.Timing)
	}
	for name, raw := range map[string]string{
		"unknown key":        `{"nope": 1}`,
		"unknown timing key": `{"timing": {"lead": 1}}`,
		"unknown block":      `{"timing": {"block_starts": {"night": "23:00"}}}`,
		"bad time":           `{"timing": {"block_starts": {"evening": "25:00"}}}`,
		"unpadded time":      `{"timing": {"block_starts": {"evening": "9:00"}}}`,
		"no room to confirm": `{"timing": {"signup_days": 12}}`,
		"zero lead":          `{"timing": {"lead_days": 0}}`,
		"bad value":          `{"min_group_size": 1}`,
	} {
		if _, err := ParseConfig([]byte(raw)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
