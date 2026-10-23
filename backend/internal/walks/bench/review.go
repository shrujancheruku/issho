package bench

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ReviewPair is one blind pair on the human review sheet: which variant is
// plan A is kept in review_key.json, never on the page.
type ReviewPair struct {
	ID      string   `json:"id"`
	Case    string   `json:"case"`
	A       string   `json:"a"` // variant shown as plan A (key only)
	B       string   `json:"b"`
	Context string   `json:"-"`
	WalkA   *WalkOut `json:"-"`
	WalkB   *WalkOut `json:"-"`
}

// Review writes review.html (n blind pairs, sides shuffled, for a person to
// pick the better walk) and review_key.json (which variant was which).
// Answers come back as a JSON file the page downloads; Summarize reads it
// to report human preferences and how well each judge agrees with them.
//
// Each plan links to its walking route in Google Maps (by place id, no key
// needed), and embeds it when embedKey (a key with the Maps Embed API) is
// given. The reviewer sees Google's map, as anyone using Google Maps would;
// the judges never do. If review_key.json exists, its pairs, sides and ids
// are kept, so answers already given stay valid.
func Review(dir string, context map[string]string, n int, seed uint64, embedKey string) error {
	results, err := ReadResults(filepath.Join(dir, "results.jsonl"))
	if err != nil {
		return err
	}
	pairs := Pairs(results)
	page := reviewData{EmbedKey: embedKey}
	if kept, err := keptPairs(dir, pairs, context); err != nil {
		return err
	} else if kept != nil {
		page.Pairs = kept
		return writeReview(dir, page)
	}
	var all []ReviewPair
	cases := make([]string, 0, len(pairs))
	for c := range pairs {
		cases = append(cases, c)
	}
	slices.Sort(cases)
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	for _, c := range cases {
		for _, p := range pairs[c] {
			a, b := p[0], p[1]
			if rng.IntN(2) == 1 {
				a, b = b, a
			}
			all = append(all, ReviewPair{Case: c, A: a.Variant, B: b.Variant, Context: context[c], WalkA: a.Walk, WalkB: b.Walk})
		}
	}
	rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	if n > 0 && n < len(all) {
		all = all[:n]
	}
	for i := range all {
		all[i].ID = fmt.Sprintf("p%03d", i+1)
	}
	key, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "review_key.json"), key, 0o644); err != nil {
		return err
	}
	page.Pairs = all
	return writeReview(dir, page)
}

type reviewData struct {
	Pairs    []ReviewPair
	EmbedKey string
}

func writeReview(dir string, page reviewData) error {
	f, err := os.Create(filepath.Join(dir, "review.html"))
	if err != nil {
		return err
	}
	defer f.Close()
	return reviewPage.Execute(f, page)
}

// keptPairs rebuilds an existing sheet's pairs (same ids and sides) with
// their walks; nil when there's no sheet yet.
func keptPairs(dir string, pairs map[string][][2]Result, context map[string]string) ([]ReviewPair, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "review_key.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var key []ReviewPair
	if err := json.Unmarshal(raw, &key); err != nil {
		return nil, err
	}
	walk := func(c, v string) *WalkOut {
		for _, p := range pairs[c] {
			for _, r := range p {
				if r.Variant == v {
					return r.Walk
				}
			}
		}
		return nil
	}
	for i := range key {
		p := &key[i]
		p.Context, p.WalkA, p.WalkB = context[p.Case], walk(p.Case, p.A), walk(p.Case, p.B)
		if p.WalkA == nil || p.WalkB == nil {
			return nil, fmt.Errorf("review_key.json: pair %s no longer matches the results; delete it to start a new sheet", p.ID)
		}
	}
	return key, nil
}

// routeURL opens a walk in Google Maps with walking directions through
// every stop in order (Maps URLs: no key; places by id, names as labels).
func routeURL(w *WalkOut) string {
	if len(w.Stops) < 2 {
		return ""
	}
	first, last := w.Stops[0], w.Stops[len(w.Stops)-1]
	q := url.Values{
		"api": {"1"}, "travelmode": {"walking"},
		"origin": {first.Name}, "origin_place_id": {first.PlaceID},
		"destination": {last.Name}, "destination_place_id": {last.PlaceID},
	}
	var names, ids []string
	for _, s := range w.Stops[1 : len(w.Stops)-1] {
		names, ids = append(names, s.Name), append(ids, s.PlaceID)
	}
	if len(ids) > 0 {
		q.Set("waypoints", strings.Join(names, "|"))
		q.Set("waypoint_place_ids", strings.Join(ids, "|"))
	}
	return "https://www.google.com/maps/dir/?" + q.Encode()
}

// embedURL is the same route for an embedded map (Maps Embed API).
func embedURL(key string, w *WalkOut) string {
	if key == "" || len(w.Stops) < 2 {
		return ""
	}
	q := url.Values{
		"key": {key}, "mode": {"walking"},
		"origin":      {"place_id:" + w.Stops[0].PlaceID},
		"destination": {"place_id:" + w.Stops[len(w.Stops)-1].PlaceID},
	}
	var mid []string
	for _, s := range w.Stops[1 : len(w.Stops)-1] {
		mid = append(mid, "place_id:"+s.PlaceID)
	}
	if len(mid) > 0 {
		q.Set("waypoints", strings.Join(mid, "|"))
	}
	return "https://www.google.com/maps/embed/v1/directions?" + q.Encode()
}

var reviewPage = template.Must(template.New("review").Funcs(template.FuncMap{
	"kind":  func(s string) string { return strings.ReplaceAll(s, "_", " ") },
	"route": func(w *WalkOut) template.URL { return template.URL(routeURL(w)) },
	"embed": func(key string, w *WalkOut) template.URL { return template.URL(embedURL(key, w)) },
}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Walk review</title>
<style>
:root { --bg:#fff; --fg:#1a1a1a; --muted:#666; --line:#ddd; --accent:#bf1e2e; }
@media (prefers-color-scheme: dark) { :root { --bg:#111; --fg:#eee; --muted:#999; --line:#333; } }
body { background:var(--bg); color:var(--fg); font:15px/1.5 system-ui, sans-serif; margin:0 auto; max-width:1100px; padding:16px; }
.pair { border-top:1px solid var(--line); padding:24px 0; }
.ctx { color:var(--muted); white-space:pre-wrap; font-size:13px; }
.plans { display:grid; grid-template-columns:1fr 1fr; gap:16px; }
@media (max-width:700px) { .plans { grid-template-columns:1fr; } }
.plan h3 { margin:8px 0 4px; } .plan ol { padding-left:20px; } .plan li { margin:6px 0; }
.meta { color:var(--muted); font-size:13px; }
fieldset { border:1px solid var(--line); margin-top:12px; } textarea { width:100%; }
.map { width:100%; height:260px; border:0; border-radius:8px; } a { color:var(--accent); }
button { background:var(--accent); color:#fff; border:0; padding:10px 16px; border-radius:6px; font-size:15px; }
#bar { position:sticky; top:0; background:var(--bg); padding:8px 0; border-bottom:1px solid var(--line); }
</style></head><body>
<div id="bar"><b>Walk review</b>: which outing would you rather go on? <span id="count"></span> <button onclick="save()">Download answers</button></div>
<p>Each pair is two plans for the same area, theme and month, from different models, in random order. Judge them as a member of the group would. Each plan's link opens its walking route through every stop in Google Maps (on a phone, Google Maps may show only the first few stops).</p>
{{$key := .EmbedKey}}{{range .Pairs}}<div class="pair" data-id="{{.ID}}">
<h2>{{.ID}} · {{.Case}}</h2><div class="ctx">{{.Context}}</div>
<div class="plans">
<div class="plan"><h3>A: {{.WalkA.Title}}</h3><p>{{.WalkA.Summary}}</p>
{{if $key}}<iframe class="map" loading="lazy" referrerpolicy="no-referrer-when-downgrade" src="{{embed $key .WalkA}}"></iframe>{{end}}
<p><a href="{{route .WalkA}}" target="_blank" rel="noopener">Walking route in Google Maps ↗</a></p>
<ol>{{range .WalkA.Stops}}<li><b>{{.Name}}</b> <span class="meta">{{.Role}} · {{kind .Kind}} · {{.Minutes}} min</span><br>{{.Note}}</li>{{end}}</ol></div>
<div class="plan"><h3>B: {{.WalkB.Title}}</h3><p>{{.WalkB.Summary}}</p>
{{if $key}}<iframe class="map" loading="lazy" referrerpolicy="no-referrer-when-downgrade" src="{{embed $key .WalkB}}"></iframe>{{end}}
<p><a href="{{route .WalkB}}" target="_blank" rel="noopener">Walking route in Google Maps ↗</a></p>
<ol>{{range .WalkB.Stops}}<li><b>{{.Name}}</b> <span class="meta">{{.Role}} · {{kind .Kind}} · {{.Minutes}} min</span><br>{{.Note}}</li>{{end}}</ol></div>
</div>
<fieldset><legend>Better overall</legend>
<label><input type="radio" name="{{.ID}}" value="A"> A</label>
<label><input type="radio" name="{{.ID}}" value="B"> B</label>
<label><input type="radio" name="{{.ID}}" value="tie"> About the same</label>
<label><input type="checkbox" name="{{.ID}}-wrong"> A place or fact is wrong</label>
<textarea name="{{.ID}}-note" rows="2" placeholder="Why (optional)"></textarea></fieldset>
</div>{{end}}
<script>
const KEY = "walk-review-answers";
function collect() {
  const out = {};
  document.querySelectorAll(".pair").forEach(p => {
    const id = p.dataset.id, pick = p.querySelector('input[type=radio]:checked');
    if (pick) out[id] = { overall: pick.value, wrong: p.querySelector('input[type=checkbox]').checked, note: p.querySelector('textarea').value };
  });
  return out;
}
function update() {
  const a = collect();
  document.getElementById("count").textContent = Object.keys(a).length + " of " + document.querySelectorAll(".pair").length + " answered";
  try { localStorage.setItem(KEY, JSON.stringify(a)); } catch (e) {}
}
function save() {
  const blob = new Blob([JSON.stringify(collect(), null, 2)], { type: "application/json" });
  const a = document.createElement("a"); a.href = URL.createObjectURL(blob); a.download = "review_answers.json"; a.click();
}
try {
  const saved = JSON.parse(localStorage.getItem(KEY) || "{}");
  for (const [id, v] of Object.entries(saved)) {
    const r = document.querySelector('input[name="' + id + '"][value="' + v.overall + '"]'); if (r) r.checked = true;
    const c = document.querySelector('input[name="' + id + '-wrong"]'); if (c) c.checked = v.wrong;
    const t = document.querySelector('textarea[name="' + id + '-note"]'); if (t) t.value = v.note || "";
  }
} catch (e) {}
document.addEventListener("change", update); document.addEventListener("input", update); update();
</script></body></html>`))
