package cities

// The rules for editing a city pack from the admin SPA's editor: what is
// kept from the file, what must hold before a change is proposed, and the
// words of the pull request. citypacks.Handler checks the caller and talks
// to GitHub; everything here works on packs and the database only.

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	gmaps "github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

// Compiled once: CheckPack runs on every proposal and apply.
var (
	cityID   = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)
	currency = regexp.MustCompile(`^[A-Z]{3}$`)
)

// CheckPack validates a pack as CI does, plus what CI can't know: the time
// zone loads and the currency looks like one.
func CheckPack(ctx context.Context, db *pgxpool.Pool, p Pack) error {
	if !cityID.MatchString(p.ID) {
		return errs.Invalid("id: lowercase letters, digits and dashes, starting with a letter")
	}
	acts, err := CatalogActivities(ctx, db)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, a := range acts {
		ids[a.ID] = true
	}
	if err := ValidatePack(p, ids); err != nil {
		return errs.Invalid("%v", err)
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil || !strings.Contains(p.Timezone, "/") {
		return errs.Invalid("timezone %q: an IANA name like Asia/Tokyo", p.Timezone)
	}
	if !currency.MatchString(p.Currency) {
		return errs.Invalid("currency %q: an ISO 4217 code like JPY", p.Currency)
	}
	return nil
}

// ErrNothingChanged is Propose's refusal of a pack equal to the default
// branch's (an errs.Invalid).
var ErrNothingChanged = errs.Invalid("nothing changed")

// Proposal is an edited pack ready to propose: the file's bytes and the
// pull request's title and body.
type Proposal struct {
	Pack    Pack
	Content []byte
	Title   string
	Body    string
}

// Propose turns the editor's pack p into a proposal against base, the default
// branch's pack (nil for a new city): it keeps what the editor doesn't show,
// checks the result, and refuses a change that removes an area or changes
// nothing. note is the admin's free text for the pull request.
func Propose(ctx context.Context, db *pgxpool.Pool, p Pack, base *Pack, note string) (Proposal, error) {
	var b Pack
	if base != nil {
		b = *base
	}
	keepFileOnly(&p, b)
	keepOverrideNames(&p, b)
	if err := CheckPack(ctx, db, p); err != nil {
		return Proposal{}, err
	}
	if base != nil {
		// Area ids are referenced by profiles, plans and venues: an area
		// can be renamed or moved, never removed (seeding keeps it anyway).
		for _, a := range base.Areas {
			if !slices.ContainsFunc(p.Areas, func(b Area) bool { return b.ID == a.ID }) {
				return Proposal{}, errs.Invalid("area %s: areas can't be removed or have their id changed", a.ID)
			}
		}
	}
	changes := Changes(base, p)
	if len(changes) == 0 {
		return Proposal{}, ErrNothingChanged
	}
	content, err := EncodePack(p)
	if err != nil {
		return Proposal{}, err
	}
	name := p.Name["en"]
	title := fmt.Sprintf("feat(data): update the %s city pack", name)
	if base == nil {
		title = fmt.Sprintf("feat(data): add the %s city pack", name)
	}
	var body strings.Builder
	// The repository is public: the pull request says what changed, never
	// who proposed it.
	body.WriteString("Proposed from the admin SPA's city-pack editor.\n\n")
	if note := strings.TrimSpace(note); note != "" {
		fmt.Fprintf(&body, "%s\n\n", note)
	}
	body.WriteString("**Changes**\n")
	for _, ch := range changes {
		fmt.Fprintf(&body, "- %s\n", ch)
	}
	body.WriteString("\n**After merging:** apply the pack on the admin SPA's Cities page (it seeds the database and rebuilds travel times when areas changed), then find venues for new or moved areas.\n")
	return Proposal{Pack: p, Content: content, Title: title, Body: body.String()}, nil
}

// Changes describes a pack's changes from the default branch's (nil for a
// new city), in words for the pull request; none = nothing changed.
func Changes(old *Pack, p Pack) []string {
	if old == nil {
		return []string{fmt.Sprintf("New city %s (%s): %d areas, languages %s, %s, %s",
			p.Name["en"], p.ID, len(p.Areas), strings.Join(p.Languages, ", "), p.Timezone, p.Currency)}
	}
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	if !maps.Equal(old.Name, p.Name) {
		add("Name: %v → %v", old.Name, p.Name)
	}
	if old.Timezone != p.Timezone {
		add("Time zone: %s → %s", old.Timezone, p.Timezone)
	}
	if old.Currency != p.Currency {
		add("Currency: %s → %s", old.Currency, p.Currency)
	}
	if !slices.Equal(old.Languages, p.Languages) {
		add("Languages: %s → %s", strings.Join(old.Languages, ", "), strings.Join(p.Languages, ", "))
	}
	if !slices.Equal(old.TravelModes, p.TravelModes) {
		add("Travel modes: %s → %s", strings.Join(old.TravelModes, ", "), strings.Join(p.TravelModes, ", "))
	}
	if old.Comment != p.Comment {
		add("Comment changed")
	}
	oldAreas := map[string]Area{}
	for _, a := range old.Areas {
		oldAreas[a.ID] = a
	}
	for _, a := range p.Areas {
		o, ok := oldAreas[a.ID]
		switch {
		case !ok:
			add("New area %s (%s) at %.5f, %.5f", a.Name["en"], a.ID, a.Lat, a.Lng)
		default:
			if !maps.Equal(o.Name, a.Name) {
				add("Area %s renamed: %v → %v", a.ID, o.Name, a.Name)
			}
			if d := gmaps.DistanceM(gmaps.LatLng{Lat: o.Lat, Lng: o.Lng}, gmaps.LatLng{Lat: a.Lat, Lng: a.Lng}); d > 0 {
				add("Area %s moved %.0f m", a.ID, d)
			}
		}
	}
	oldOv := map[string]ActivityOverride{}
	for _, o := range old.ActivityOverrides {
		oldOv[o.ActivityID] = o
	}
	seen := map[string]bool{}
	for _, o := range p.ActivityOverrides {
		seen[o.ActivityID] = true
		was, ok := oldOv[o.ActivityID]
		if !ok || was.Disabled != o.Disabled || !maps.Equal(was.Name, o.Name) || !slices.Equal(was.Months, o.Months) {
			add("Activity %s: %s", o.ActivityID, describe(o))
		}
	}
	for _, o := range old.ActivityOverrides {
		if !seen[o.ActivityID] {
			add("Activity %s: back to the catalogue's defaults", o.ActivityID)
		}
	}
	return out
}

func describe(o ActivityOverride) string {
	if o.Disabled {
		return "not offered"
	}
	var parts []string
	if o.Name["en"] != "" {
		parts = append(parts, fmt.Sprintf("called %q", o.Name["en"]))
	}
	if len(o.Months) > 0 {
		ms := make([]string, len(o.Months))
		for i, m := range o.Months {
			ms[i] = time.Month(m).String()[:3]
		}
		parts = append(parts, "in season "+strings.Join(ms, ", "))
	}
	if len(parts) == 0 {
		return "offered, as in the catalogue"
	}
	return strings.Join(parts, ", ")
}

// keepFileOnly copies the fields the editor doesn't show from the default
// branch's pack, or its pull requests would drop them: each area's osm_id and
// boundary (`tool area-boundaries` writes them) and replaces (written by hand
// when an area is retired). An area moved out of its boundary then fails
// validation, which is the point. The pack's matching overlay and
// meeting_places are hand edits in a pull request too.
func keepFileOnly(p *Pack, base Pack) {
	p.Matching, p.MeetingPlaces = base.Matching, base.MeetingPlaces
	for i, a := range p.Areas {
		if j := slices.IndexFunc(base.Areas, func(b Area) bool { return b.ID == a.ID }); j >= 0 {
			p.Areas[i].OSMID, p.Areas[i].Boundary, p.Areas[i].Replaces = base.Areas[j].OSMID, base.Areas[j].Boundary, base.Areas[j].Replaces
		}
	}
}

// keepOverrideNames carries an override's non-English names over from the
// default branch's pack: the editor shows and sends only the English one
// (PackOverride.name), so saving would otherwise drop e.g. a Japanese name.
// They are kept only while the override still has an English name; clearing
// it means the admin is removing the custom name, and an English-less name
// would fail validation. The editor's English name always wins.
func keepOverrideNames(p *Pack, base Pack) {
	for i, o := range p.ActivityOverrides {
		if o.Name["en"] == "" {
			continue
		}
		j := slices.IndexFunc(base.ActivityOverrides, func(b ActivityOverride) bool { return b.ActivityID == o.ActivityID })
		if j < 0 {
			continue
		}
		for lang, name := range base.ActivityOverrides[j].Name {
			if _, ok := o.Name[lang]; !ok {
				p.ActivityOverrides[i].Name[lang] = name
			}
		}
	}
}
