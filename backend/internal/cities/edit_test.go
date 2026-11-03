package cities

import (
	"slices"
	"strings"
	"testing"
)

func TestChanges(t *testing.T) {
	old := Pack{
		ID: "x", Name: Text{"en": "X"}, Timezone: "Asia/Tokyo", Currency: "JPY", Languages: []string{"ja"}, TravelModes: []string{"walk"},
		Areas:             []Area{{ID: "x-a", Name: Text{"en": "A"}, Lat: 35, Lng: 139}},
		ActivityOverrides: []ActivityOverride{{ActivityID: "hanami", Months: []int16{3, 4}}, {ActivityID: "surf", Disabled: true}},
	}
	if got := Changes(&old, old); len(got) != 0 {
		t.Errorf("no change: %v", got)
	}
	p := old
	p.Languages = []string{"ja", "en"}
	p.Areas = []Area{{ID: "x-a", Name: Text{"en": "A2"}, Lat: 35.001, Lng: 139}, {ID: "x-b", Name: Text{"en": "B"}, Lat: 35.1, Lng: 139.1}}
	p.ActivityOverrides = []ActivityOverride{{ActivityID: "hanami", Months: []int16{4}}, {ActivityID: "karaoke", Name: Text{"en": "Sing"}}}
	want := []string{
		"Languages: ja → ja, en",
		"Area x-a renamed: map[en:A] → map[en:A2]",
		"Area x-a moved 111 m",
		"New area B (x-b) at 35.10000, 139.10000",
		"Activity hanami: in season Apr",
		`Activity karaoke: called "Sing"`,
		"Activity surf: back to the catalogue's defaults",
	}
	got := Changes(&old, p)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := Changes(nil, p); len(got) != 1 || !strings.HasPrefix(got[0], "New city X (x): 2 areas") {
		t.Errorf("new city: %v", got)
	}
}

func TestKeepFileOnly(t *testing.T) {
	base := Pack{Matching: []byte(`{"plans_per_pool": 3}`), MeetingPlaces: []MeetingPlace{{Types: []string{"park"}, RadiusM: 500}},
		Areas: []Area{{ID: "x-a", OSMID: 7, Boundary: []string{"_p~iF~ps|U_ulLnnqC_mqNvxq`@"}, Replaces: []string{"x-old"}}}}
	// What the editor sends: no osm_id, boundary or replaces, plus a new area.
	p := Pack{Areas: []Area{{ID: "x-a", Lat: 1}, {ID: "x-new", Lat: 2}}}
	keepFileOnly(&p, base)
	if string(p.Matching) != `{"plans_per_pool": 3}` || len(p.MeetingPlaces) != 1 || p.MeetingPlaces[0].RadiusM != 500 {
		t.Errorf("matching and meeting_places not kept: %s %+v", p.Matching, p.MeetingPlaces)
	}
	if a := p.Areas[0]; a.OSMID != 7 || len(a.Boundary) != 1 || !slices.Equal(a.Replaces, []string{"x-old"}) || a.Lat != 1 {
		t.Errorf("existing area: %+v", a)
	}
	if a := p.Areas[1]; a.OSMID != 0 || a.Boundary != nil || a.Replaces != nil {
		t.Errorf("new area got a boundary: %+v", a)
	}
}

func TestKeepOverrideNames(t *testing.T) {
	base := Pack{ActivityOverrides: []ActivityOverride{
		{ActivityID: "karaoke", Name: Text{"en": "Sing", "ja": "カラオケ"}},
		{ActivityID: "hanami", Name: Text{"en": "Blossoms", "ja": "花見"}},
		{ActivityID: "surf", Disabled: true},
	}}
	// What the editor sends: English names only.
	p := Pack{ActivityOverrides: []ActivityOverride{
		{ActivityID: "karaoke", Name: Text{"en": "Sing along"}},
		{ActivityID: "hanami"}, // the English name was cleared
		{ActivityID: "surf", Disabled: true},
		{ActivityID: "new", Name: Text{"en": "New"}},
	}}
	keepOverrideNames(&p, base)
	if got := p.ActivityOverrides[0].Name; got["en"] != "Sing along" || got["ja"] != "カラオケ" || len(got) != 2 {
		t.Errorf("renamed in English: %v", got)
	}
	if got := p.ActivityOverrides[1].Name; len(got) != 0 {
		t.Errorf("name cleared: %v", got)
	}
	if got := p.ActivityOverrides[2].Name; len(got) != 0 {
		t.Errorf("no name: %v", got)
	}
	if got := p.ActivityOverrides[3].Name; len(got) != 1 {
		t.Errorf("a new override: %v", got)
	}
}
