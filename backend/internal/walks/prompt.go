package walks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SystemPrompt is the instruction every Proposer gives its model, so that
// models are compared on the same task (internal/walks/bench). Versioned:
// bump PromptVersion when changing it, so benchmark results stay comparable.
const SystemPrompt = `You plan walking outings for Issho, an app that brings small groups of strangers (aged 18 to 40) together to make friends. Plan one outing in the given area, on the given theme.

The outing:
- lasts 2 to 4 hours in all, including walking (about 3 hours is best)
- starts at a meeting place that is easy to find and gather at: a station or a cafe right by a station
- then visits 4 to 7 places that fit the theme, along one walking route with no backtracking; walks between places of 3 to 15 minutes, 2 to 5 km of walking in all; every place within about 2 km of the area's centre
- ends at a relaxed place to sit and talk (a cafe, tea house, bar or casual restaurant) close to the last place

Every place must be real, open to the public, and well known enough to find on Google Maps by its name. Never invent a place, and never include one you aren't sure still exists. Give each place's usual name as it appears on Google Maps, its kind, and a search text that finds it on Google Maps: the name plus the neighbourhood it is in (for example "Kayaba Coffee, Yanaka"), so that a place with a common name isn't mistaken for another. Choose places that make the theme feel full and varied (for a temple walk, several different temples and shrines, not one; for a photo walk, a mix of viewpoints, old streets, gardens and details worth photographing) and that suit the month and the part of the day. The date isn't fixed yet, so avoid places that are only open on some days or need booking.

For each place give the minutes spent there and a note of one or two sentences on why it's worth the stop and what to look at or do, written to the group, in plain English that machine translation handles well (no idioms or wordplay). Write everything in English and in the Latin alphabet only, with place names romanised (the app translates and shows Google's local names separately). For the meeting place, the note says where exactly to gather. The title is short and specific (at most 60 characters) and doesn't name the city, for example "Yanaka's temple lanes, ending with tea". The summary is one or two sentences on what the outing is like.`

// PromptVersion names SystemPrompt and UserPrompt's wording for benchmark
// results.
const PromptVersion = "walk-v1"

// UserPrompt is the user turn: the request, and what was wrong with the
// previous proposal (only the model's own place names and our rules).
func UserPrompt(req Request, feedback []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Theme: %s (%s)\n", req.Activity, req.Theme)
	fmt.Fprintf(&b, "Area: %s, %s (centre %.4f, %.4f)\n", req.Area, req.City, req.Center.Lat, req.Center.Lng)
	fmt.Fprintf(&b, "Month: %s\n", req.Month)
	if len(req.Blocks) == 0 || len(req.Blocks) == 3 {
		b.WriteString("Part of the day: any (morning, afternoon or evening)\n")
	} else {
		fmt.Fprintf(&b, "Part of the day: %s\n", strings.Join(req.Blocks, " or "))
	}
	if len(feedback) > 0 {
		b.WriteString("\nYour previous plan for this outing couldn't be used:\n")
		for _, f := range feedback {
			b.WriteString("- " + f + "\n")
		}
		b.WriteString("Plan it again, fixing these. Replace places that couldn't be found or used with other real places you are sure of.\n")
	}
	return b.String()
}

// ProposalSchema is the answer's JSON schema. strict adds
// "additionalProperties": false to every object, which Claude's structured
// outputs require and Gemini on Agent Platform rejects.
func ProposalSchema(strict bool) map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		o := map[string]any{"type": "object", "properties": props, "required": required}
		if strict {
			o["additionalProperties"] = false
		}
		return o
	}
	place := func() map[string]any {
		return obj(map[string]any{
			"name":    map[string]any{"type": "string"},
			"search":  map[string]any{"type": "string"},
			"kind":    map[string]any{"type": "string", "enum": Kinds},
			"minutes": map[string]any{"type": "integer"},
			"note":    map[string]any{"type": "string"},
		}, "name", "search", "kind", "minutes", "note")
	}
	return obj(map[string]any{
		"title":   map[string]any{"type": "string"},
		"summary": map[string]any{"type": "string"},
		"meet":    place(),
		"stops":   map[string]any{"type": "array", "items": place()},
		"finish":  place(),
	}, "title", "summary", "meet", "stops", "finish")
}

// ParseProposal decodes an answer in ProposalSchema's shape.
func ParseProposal(answer string) (Proposal, error) {
	type place struct {
		Name    string `json:"name"`
		Search  string `json:"search"`
		Kind    string `json:"kind"`
		Minutes int    `json:"minutes"`
		Note    string `json:"note"`
	}
	var out struct {
		Title   string  `json:"title"`
		Summary string  `json:"summary"`
		Meet    place   `json:"meet"`
		Stops   []place `json:"stops"`
		Finish  place   `json:"finish"`
	}
	if err := json.Unmarshal([]byte(answer), &out); err != nil {
		return Proposal{}, fmt.Errorf("decoding walk: %w", err)
	}
	w := Proposal{Title: out.Title, Summary: out.Summary, Meet: Place(out.Meet), Finish: Place(out.Finish)}
	for _, s := range out.Stops {
		w.Stops = append(w.Stops, Place(s))
	}
	return w, nil
}
