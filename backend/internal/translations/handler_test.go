package translations

import (
	"context"
	"encoding/json"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// fakeRepo is a repository in memory: branches of files, and pull requests.
type fakeRepo struct {
	branches map[string]map[string][]byte
	prs      []github.PR
	commits  []string
	conflict int // how many commits fail with ErrConflict first
}

func newRepo(mainFiles map[string]string) *fakeRepo {
	main := map[string][]byte{}
	for p, c := range mainFiles {
		main[p] = []byte(c)
	}
	return &fakeRepo{branches: map[string]map[string][]byte{"main": main}}
}

func (r *fakeRepo) File(_ context.Context, path, branch string) ([]byte, string, error) {
	b, ok := r.branches[branch][path]
	if !ok {
		return nil, "", github.ErrNotFound
	}
	return b, "sha", nil
}

func (r *fakeRepo) OpenPRs(_ context.Context, prefix string) ([]github.PR, error) {
	var out []github.PR
	for _, pr := range r.prs {
		if strings.HasPrefix(pr.Branch, prefix) {
			out = append(out, pr)
		}
	}
	return out, nil
}

func (r *fakeRepo) CreateBranch(_ context.Context, branch string) error {
	r.branches[branch] = maps.Clone(r.branches["main"])
	return nil
}

func (r *fakeRepo) CommitFiles(_ context.Context, branch string, files map[string][]byte, message string) error {
	if r.conflict > 0 {
		r.conflict--
		return github.ErrConflict
	}
	for p, c := range files {
		r.branches[branch][p] = c
	}
	r.commits = append(r.commits, message)
	return nil
}

func (r *fakeRepo) OpenPR(_ context.Context, branch, title, _ string) (github.PR, error) {
	pr := github.PR{Number: 100 + len(r.prs), Title: title, URL: "https://github.test/pr", Branch: branch}
	r.prs = append(r.prs, pr)
	return pr, nil
}

// memCache is app.translations for the machine translations.
type memCache struct {
	mu   sync.Mutex
	rows map[string]string
}

func (m *memCache) GetTranslations(_ context.Context, arg dbgen.GetTranslationsParams) ([]dbgen.GetTranslationsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []dbgen.GetTranslationsRow
	for _, h := range arg.SourceHashes {
		if t, ok := m.rows[arg.Lang+"/"+h]; ok {
			out = append(out, dbgen.GetTranslationsRow{SourceHash: h, Text: t})
		}
	}
	return out, nil
}

func (m *memCache) InsertTranslation(_ context.Context, arg dbgen.InsertTranslationParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[arg.Lang+"/"+arg.SourceHash] = arg.Text
	return nil
}

type tagger struct{}

func (tagger) Name() string { return "tagger" }
func (tagger) Translate(_ context.Context, lang string, items []translate.Item) ([]string, error) {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "[" + lang + "] " + it.Text
	}
	return out, nil
}

// store is the catalogue part of app.translations.
type store struct {
	rows     map[string]dbgen.GetTranslationRow // lang/hash
	reviewed []dbgen.ReviewTranslationParams
}

func (s *store) ListTranslationLanguages(context.Context) ([]dbgen.ListTranslationLanguagesRow, error) {
	return []dbgen.ListTranslationLanguagesRow{{Lang: "es", Total: 3}, {Lang: "ja", Total: 2}}, nil
}

func (s *store) ListCatalogLanguages(_ context.Context, ui string) ([]dbgen.ListCatalogLanguagesRow, error) {
	if ui != uistrings.ContextPrefix {
		panic("wrong UI prefix")
	}
	return []dbgen.ListCatalogLanguagesRow{{Lang: "es", Total: 1}, {Lang: "ja", Total: 1, Unreviewed: 1}}, nil
}

func (s *store) ListCatalogTranslations(_ context.Context, arg dbgen.ListCatalogTranslationsParams) ([]dbgen.ListCatalogTranslationsRow, error) {
	var out []dbgen.ListCatalogTranslationsRow
	for _, r := range s.rows {
		if r.Lang == arg.Lang && !strings.HasPrefix(r.Context, arg.UiContext) {
			out = append(out, dbgen.ListCatalogTranslationsRow(r))
		}
	}
	return out, nil
}

func (s *store) GetTranslation(_ context.Context, arg dbgen.GetTranslationParams) (dbgen.GetTranslationRow, error) {
	r, ok := s.rows[arg.Lang+"/"+arg.SourceHash]
	if !ok {
		return r, pgx.ErrNoRows
	}
	return r, nil
}

func (s *store) ReviewTranslation(_ context.Context, arg dbgen.ReviewTranslationParams) (int64, error) {
	s.reviewed = append(s.reviewed, arg)
	r := s.rows[arg.Lang+"/"+arg.SourceHash]
	r.Text, r.Reviewed = arg.Text, true
	s.rows[arg.Lang+"/"+arg.SourceHash] = r
	return 1, nil
}

const english = `{"hi":"Hello","who":"Signed in as {email}","bye":"Goodbye"}`

func setup(t *testing.T, mainJa string) (*Handler, *fakeRepo) {
	t.Helper()
	human := map[string][]byte{
		"en": []byte(`{"hi":{"text":"Hey there","source":"` + uistrings.SourceHash("Hello") + `"}}`),
		"ja": []byte(mainJa),
	}
	cat, err := uistrings.New([]byte(english), human, map[string]string{"hi": "A greeting."})
	if err != nil {
		t.Fatal(err)
	}
	repo := newRepo(map[string]string{uistrings.HumanDir + "/ja.json": mainJa})
	h := &Handler{
		Q: &store{rows: map[string]dbgen.GetTranslationRow{
			"ja/ui":  {SourceHash: "ui", Lang: "ja", Source: "Save", Context: uistrings.ContextPrefix + ". A button.", Text: "節約"},
			"ja/act": {SourceHash: "act", Lang: "ja", Source: "Pottery class", Context: "name of a group activity", Text: "陶芸"},
		}},
		Catalog:      cat,
		Translations: &translate.Service{Store: &memCache{rows: map[string]string{}}, Translator: tagger{}},
		GitHub:       repo,
		Base:         "main",
		Now:          func() time.Time { return time.Unix(1700000000, 0) },
	}
	return h, repo
}

var (
	admin      = auth.Caller{UserID: uuid.New(), Admin: true}
	japanese   = auth.Caller{UserID: uuid.New(), TranslatorLangs: []string{"ja"}}
	nobody     = auth.Caller{UserID: uuid.New()}
	mainJaFile = `{"bye":{"text":"さようなら","source":"` + uistrings.SourceHash("Good-bye") + `"},` +
		`"who":{"text":"{email} でログイン中","source":"` + uistrings.SourceHash("Signed in as {email}") + `"}}`
)

func as(c auth.Caller) context.Context { return auth.WithCaller(context.Background(), c) }

func TestPermissions(t *testing.T) {
	h, _ := setup(t, mainJaFile)
	list := func(c auth.Caller, lang string) connect.Code {
		_, err := h.ListAppStrings(as(c), connect.NewRequest(&isshov1.ListAppStringsRequest{Lang: lang}))
		return connect.CodeOf(err)
	}
	if got := list(japanese, "es"); got != connect.CodePermissionDenied {
		t.Errorf("another language: %v", got)
	}
	if got := list(nobody, "ja"); got != connect.CodePermissionDenied {
		t.Errorf("not a translator: %v", got)
	}
	if got := list(admin, "xx"); got != connect.CodeInvalidArgument {
		t.Errorf("bad code: %v", got)
	}
	if _, err := h.ListAppStrings(as(japanese), connect.NewRequest(&isshov1.ListAppStringsRequest{Lang: "ja"})); err != nil {
		t.Errorf("own language: %v", err)
	}
	_, err := h.SaveAppString(as(japanese), connect.NewRequest(&isshov1.SaveAppStringRequest{Lang: "es", Key: "hi", Text: "Hola"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("save another language: %v", err)
	}
	_, err = h.ReviewCatalogTranslation(as(japanese), connect.NewRequest(&isshov1.ReviewCatalogTranslationRequest{Lang: "es", SourceHash: "act", Text: "x"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("review another language: %v", err)
	}

	me, err := h.GetTranslator(as(japanese), connect.NewRequest(&isshov1.GetTranslatorRequest{}))
	if err != nil || me.Msg.Admin || len(me.Msg.Languages) != 1 || me.Msg.Languages[0] != "ja" || !me.Msg.CanPropose {
		t.Errorf("GetTranslator: %+v %v", me, err)
	}
}

func TestListAppLanguages(t *testing.T) {
	h, repo := setup(t, mainJaFile)
	repo.prs = []github.PR{{Number: 7, URL: "u", Branch: "translations/ja-1"}}
	repo.branches["translations/ja-1"] = repo.branches["main"]
	res, err := h.ListAppLanguages(as(japanese), connect.NewRequest(&isshov1.ListAppLanguagesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Msg.Languages) != 1 {
		t.Fatalf("a translator sees their languages only: %+v", res.Msg.Languages)
	}
	ja := res.Msg.Languages[0]
	// "who" is current; "bye" was written for older English.
	if ja.Lang != "ja" || ja.Human != 1 || ja.Stale != 1 || ja.Total != 3 || ja.PullRequest.GetNumber() != 7 {
		t.Errorf("ja: %+v", ja)
	}
	res, err = h.ListAppLanguages(as(admin), connect.NewRequest(&isshov1.ListAppLanguagesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var langs []string
	for _, l := range res.Msg.Languages {
		langs = append(langs, l.Lang)
	}
	// Human files (en, ja) and cached machine translations (es).
	if strings.Join(langs, ",") != "en,es,ja" {
		t.Errorf("admin languages %v", langs)
	}
}

func rows(t *testing.T, h *Handler, lang string) (map[string]*isshov1.AppString, *isshov1.ListAppStringsResponse) {
	t.Helper()
	res, err := h.ListAppStrings(as(admin), connect.NewRequest(&isshov1.ListAppStringsRequest{Lang: lang}))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*isshov1.AppString{}
	for _, s := range res.Msg.Strings {
		out[s.Key] = s
	}
	return out, res.Msg
}

func TestSaveAndRemove(t *testing.T) {
	h, repo := setup(t, mainJaFile)
	ctx := as(japanese)

	got, res := rows(t, h, "ja")
	if !res.Complete || res.PullRequest != nil {
		t.Errorf("list: %+v", res)
	}
	hi := got["hi"]
	if hi.English != "Hello" || hi.EnglishVoice != "Hey there" || hi.Note != "A greeting." || hi.Machine != "[ja] Hello" || hi.Human != "" || hi.Pending {
		t.Errorf("hi: %+v", hi)
	}
	if got["bye"].Human != "さようなら" || !got["bye"].HumanStale || got["who"].HumanStale {
		t.Errorf("bye/who: %+v %+v", got["bye"], got["who"])
	}

	// Saving the same text as main changes nothing: no branch, no PR.
	same, err := h.SaveAppString(ctx, connect.NewRequest(&isshov1.SaveAppStringRequest{Lang: "ja", Key: "who", Text: "{email} でログイン中"}))
	if err != nil || same.Msg.PullRequest != nil || len(repo.branches) != 1 {
		t.Fatalf("no-op save: %+v %v (branches %d)", same, err, len(repo.branches))
	}

	// The first change opens the language's pull request, committing both
	// copies of the file in the format `make gen` writes.
	saved, err := h.SaveAppString(ctx, connect.NewRequest(&isshov1.SaveAppStringRequest{Lang: "ja", Key: "hi", Text: " やあ "}))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Msg.PullRequest.GetNumber() != 100 || len(repo.prs) != 1 || repo.prs[0].Title != "feat(data): update the Japanese translations" ||
		repo.prs[0].Branch != "translations/ja-1700000000" {
		t.Errorf("pull request: %+v %+v", saved.Msg.PullRequest, repo.prs)
	}
	if s := saved.Msg.AppString; !s.Pending || s.PendingText != "やあ" || s.Machine != "[ja] Hello" {
		t.Errorf("saved row: %+v", s)
	}
	branch := repo.branches["translations/ja-1700000000"]
	file, copyFile := branch[uistrings.HumanDir+"/ja.json"], branch[uistrings.HumanCopyDir+"/ja.json"]
	if string(file) != string(copyFile) {
		t.Error("both copies must match")
	}
	stamped, err := uistrings.Stamp([]byte(english), file)
	if err != nil || string(stamped) != string(file) {
		t.Errorf("not in make gen's format:\n%s\n%v", file, err)
	}
	var m map[string]uistrings.Human
	_ = json.Unmarshal(file, &m)
	if m["hi"] != (uistrings.Human{Text: "やあ", Source: uistrings.SourceHash("Hello")}) || m["who"].Text == "" || m["bye"].Text == "" {
		t.Errorf("file: %+v", m)
	}

	// The next change goes to the same pull request, retrying once when the
	// branch moved.
	repo.conflict = 1
	if _, err := h.SaveAppString(ctx, connect.NewRequest(&isshov1.SaveAppStringRequest{Lang: "ja", Key: "bye", Text: "またね"})); err != nil {
		t.Fatal(err)
	}
	if len(repo.prs) != 1 || len(repo.commits) != 2 || repo.commits[1] != "feat(data): set the ja text of bye" {
		t.Errorf("second save: prs %d, commits %v", len(repo.prs), repo.commits)
	}
	got, res = rows(t, h, "ja")
	if res.PullRequest.GetNumber() != 100 || !got["bye"].Pending || got["bye"].PendingText != "またね" || got["who"].Pending {
		t.Errorf("after saves: %+v %+v %+v", res.PullRequest, got["bye"], got["who"])
	}

	// Removing a human version: pending with no text.
	rm, err := h.RemoveAppString(ctx, connect.NewRequest(&isshov1.RemoveAppStringRequest{Lang: "ja", Key: "who"}))
	if err != nil || !rm.Msg.AppString.Pending || rm.Msg.AppString.PendingText != "" || repo.commits[2] != "feat(data): remove the ja human version of who" {
		t.Errorf("remove: %+v %v %v", rm, err, repo.commits)
	}

	for _, bad := range []*isshov1.SaveAppStringRequest{
		{Lang: "ja", Key: "who", Text: "ログイン中"}, // lost {email}
		{Lang: "ja", Key: "nope", Text: "x"},    // no such key
		{Lang: "ja", Key: "hi", Text: "   "},    // empty
	} {
		if _, err := h.SaveAppString(ctx, connect.NewRequest(bad)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%+v: %v", bad, err)
		}
	}

	h.GitHub = nil
	if _, err := h.SaveAppString(ctx, connect.NewRequest(&isshov1.SaveAppStringRequest{Lang: "ja", Key: "hi", Text: "やっほー"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("without GitHub: %v", err)
	}
}

func TestEnglish(t *testing.T) {
	h, _ := setup(t, `{}`)
	got, _ := rows(t, h, "en")
	if got["hi"].Machine != "" || got["hi"].Human != "Hey there" || got["hi"].EnglishVoice != "Hey there" {
		t.Errorf("en hi: %+v", got["hi"])
	}
}

func TestCatalog(t *testing.T) {
	h, _ := setup(t, mainJaFile)
	langs, err := h.ListCatalogLanguages(as(japanese), connect.NewRequest(&isshov1.ListCatalogLanguagesRequest{}))
	if err != nil || len(langs.Msg.Languages) != 1 || langs.Msg.Languages[0].Lang != "ja" {
		t.Fatalf("languages: %+v %v", langs, err)
	}
	list, err := h.ListCatalogTranslations(as(japanese), connect.NewRequest(&isshov1.ListCatalogTranslationsRequest{Lang: "ja"}))
	if err != nil || len(list.Msg.Translations) != 1 || list.Msg.Translations[0].SourceHash != "act" {
		t.Fatalf("UI strings must be left out: %+v %v", list, err)
	}
	_, err = h.ReviewCatalogTranslation(as(japanese), connect.NewRequest(&isshov1.ReviewCatalogTranslationRequest{Lang: "ja", SourceHash: "ui", Text: "保存"}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a UI string through the catalogue review: %v", err)
	}
	res, err := h.ReviewCatalogTranslation(as(japanese), connect.NewRequest(&isshov1.ReviewCatalogTranslationRequest{Lang: "ja", SourceHash: "act", Text: "陶芸教室"}))
	if err != nil || !res.Msg.Translation.Reviewed || res.Msg.Translation.Text != "陶芸教室" {
		t.Errorf("review: %+v %v", res, err)
	}
}
