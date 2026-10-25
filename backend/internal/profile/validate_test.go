package profile

import (
	"errors"
	"testing"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
)

const year = 2026

func valid() Profile {
	return Profile{
		DisplayName:     "  Aiko ",
		BirthYear:       1995,
		Gender:          "woman",
		HomeAreaID:      "tokyo-kichijoji",
		MaxTravelMin:    30,
		Languages:       []Language{{Code: "JA", Fluent: true, Level: 3}, {Code: "es", Level: 1}},
		PrimaryLanguage: "ja",
		Availability:    []Slot{{6, "morning"}, {0, "evening"}},
		Prefs:           map[string]int{"photography": 2, "pottery": 1, "karaoke": 0},
		SizePref:        "medium",
		Energy:          2,
		Budget:          1,
		ConductVersion:  ConductVersion,
	}
}

func TestValidate(t *testing.T) {
	p := valid()
	if err := validate(&p, year); err != nil {
		t.Fatal(err)
	}
	if p.DisplayName != "Aiko" || p.Languages[0].Code != "ja" || p.Languages[0].Level != 0 {
		t.Errorf("not normalised: %+v", p)
	}

	for name, tc := range map[string]struct {
		edit  func(*Profile)
		field string
	}{
		"blank name": {func(p *Profile) { p.DisplayName = "   " }, "display_name"},
		"long name": {func(p *Profile) {
			p.DisplayName = "あいうえおかきくけこさしすせそたちつてとなにぬねのはひふへほまみむめもやゆよらりるれ"
		}, "display_name"},
		"17 this year":           {func(p *Profile) { p.BirthYear = year - 17 }, "birth_year"},
		"typo year":              {func(p *Profile) { p.BirthYear = 1099 }, "birth_year"},
		"no gender":              {func(p *Profile) { p.Gender = "" }, "gender"},
		"same gender, no gender": {func(p *Profile) { p.Gender, p.SameGenderOnly = "unspecified", true }, "same_gender_only"},
		"no area":                {func(p *Profile) { p.HomeAreaID = "" }, "home_area_id"},
		"travel too short":       {func(p *Profile) { p.MaxTravelMin = 4 }, "max_travel_min"},
		"no languages":           {func(p *Profile) { p.Languages = nil }, "spoken_languages"},
		"only learning":          {func(p *Profile) { p.Languages = []Language{{Code: "ja", Level: 2}} }, "spoken_languages"},
		"learner without level":  {func(p *Profile) { p.Languages[1].Level = 0 }, "spoken_languages"},
		"made-up language":       {func(p *Profile) { p.Languages[1].Code = "xx" }, "spoken_languages"},
		"language twice":         {func(p *Profile) { p.Languages[1] = Language{Code: "ja", Fluent: true} }, "spoken_languages"},
		"no primary":             {func(p *Profile) { p.PrimaryLanguage = "" }, "primary_language"},
		"secondary = primary":    {func(p *Profile) { p.SecondaryLanguage = "ja" }, "secondary_language"},
		"no availability":        {func(p *Profile) { p.Availability = nil }, "availability"},
		"bad weekday":            {func(p *Profile) { p.Availability[0].Weekday = 7 }, "availability"},
		"slot twice":             {func(p *Profile) { p.Availability[1] = p.Availability[0] }, "availability"},
		"two activities":         {func(p *Profile) { delete(p.Prefs, "karaoke") }, "activities"},
		"bad interest":           {func(p *Profile) { p.Prefs["karaoke"] = 3 }, "activities"},
		"no size":                {func(p *Profile) { p.SizePref = "" }, "group_size"},
		"energy":                 {func(p *Profile) { p.Energy = 0 }, "energy"},
		"budget":                 {func(p *Profile) { p.Budget = 4 }, "budget"},
		"old conduct":            {func(p *Profile) { p.ConductVersion = ConductVersion - 1 }, "conduct_version"},
	} {
		t.Run(name, func(t *testing.T) {
			p := valid()
			tc.edit(&p)
			err := validate(&p, year)
			var ie *InvalidError
			if !errors.As(err, &ie) || ie.Field != tc.field {
				t.Errorf("err = %v, want an InvalidError for %s", err, tc.field)
			}
		})
	}
}

func TestSecondaryOptional(t *testing.T) {
	p := valid()
	p.SecondaryLanguage = "ES"
	if err := validate(&p, year); err != nil || p.SecondaryLanguage != "es" {
		t.Errorf("err = %v, secondary = %q", err, p.SecondaryLanguage)
	}
	p = valid()
	p.Gender, p.SameGenderOnly = "nonbinary", true
	if err := validate(&p, year); err != nil {
		t.Errorf("nonbinary, same gender only: %v", err)
	}
}

func TestProtoRoundTrip(t *testing.T) {
	p := valid()
	if err := validate(&p, year); err != nil {
		t.Fatal(err)
	}
	p.CityID = "tokyo"
	back, err := fromProto(toProto(p))
	if err != nil {
		t.Fatal(err)
	}
	back.CityID = "tokyo" // read-only: not taken from requests
	m1, m2 := toProto(p), toProto(back)
	if m1.String() != m2.String() {
		t.Errorf("round trip changed the profile:\n%v\n%v", m1, m2)
	}

	dup := toProto(p)
	dup.Activities = append(dup.Activities, dup.Activities[0])
	if _, err := fromProto(dup); !IsInvalid(err) {
		t.Errorf("duplicate activity: %v", err)
	}
	noInterest := toProto(p)
	noInterest.Activities[0].Interest = isshov1.Interest_INTEREST_UNSPECIFIED
	if _, err := fromProto(noInterest); !IsInvalid(err) {
		t.Errorf("unspecified interest: %v", err)
	}
	noBlock := toProto(p)
	noBlock.Availability[0].Block = isshov1.TimeBlock_TIME_BLOCK_UNSPECIFIED
	if _, err := fromProto(noBlock); !IsInvalid(err) {
		t.Errorf("unspecified block: %v", err)
	}
	unsetGender := toProto(p)
	unsetGender.Gender = isshov1.Gender_GENDER_UNSPECIFIED
	in, _ := fromProto(unsetGender)
	if err := validate(&in, year); !IsInvalid(err) {
		t.Errorf("GENDER_UNSPECIFIED must be rejected, got %v", err)
	}
}

// Traditional Chinese is a display language (zh-Hant), from any of its tags;
// Simplified stays zh.
func TestTraditionalChinese(t *testing.T) {
	for in, want := range map[string]string{"zh-Hant": "zh-Hant", "zh-TW": "zh-Hant", "zh-hk": "zh-Hant", "zh-CN": "zh"} {
		p := valid()
		p.SecondaryLanguage = in
		if err := validate(&p, year); err != nil || p.SecondaryLanguage != want {
			t.Errorf("%s: err = %v, secondary = %q", in, err, p.SecondaryLanguage)
		}
	}
	p := valid()
	p.PrimaryLanguage = "zh-Hans-XX-junk"
	p.SecondaryLanguage = "zh"
	if err := validate(&p, year); err == nil {
		t.Error("zh twice must be refused")
	}
}
