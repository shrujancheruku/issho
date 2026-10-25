// Package translations serves TranslationService: the admin SPA's
// translation editor, for admins and translators.
//
// UI strings are edited as human versions in the repository
// (uistrings.HumanDir), like city packs (owner's decision): every save is a
// commit to one rolling pull request per language, which the owner merges;
// the API then serves it without a deploy (uistrings.Catalog.Follow).
// Catalogue text (activity and city names) has no file in the repository,
// so its machine translations are corrected in app.translations instead.
package translations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// BranchPrefix marks the editor's branches: translations/<lang>-<unix time>.
const BranchPrefix = "translations/"

// catalogPage is how many catalogue translations ListCatalogTranslations returns.
const catalogPage = 100

// Repo is the GitHub repository the human versions live in (*github.Client).
type Repo interface {
	File(ctx context.Context, path, branch string) ([]byte, string, error)
	OpenPRs(ctx context.Context, prefix string) ([]github.PR, error)
	CreateBranch(ctx context.Context, branch string) error
	CommitFiles(ctx context.Context, branch string, files map[string][]byte, message string) error
	OpenPR(ctx context.Context, branch, title, body string) (github.PR, error)
}

// Drafter proposes a better version of a string (*gemini.Translator).
type Drafter interface {
	Draft(ctx context.Context, lang string, d translate.Draft) (string, error)
}

// Store is the catalogue translations' part of the database (*dbgen.Queries).
type Store interface {
	ListTranslationLanguages(ctx context.Context) ([]dbgen.ListTranslationLanguagesRow, error)
	ListCatalogLanguages(ctx context.Context, uiContext string) ([]dbgen.ListCatalogLanguagesRow, error)
	ListCatalogTranslations(ctx context.Context, arg dbgen.ListCatalogTranslationsParams) ([]dbgen.ListCatalogTranslationsRow, error)
	GetTranslation(ctx context.Context, arg dbgen.GetTranslationParams) (dbgen.GetTranslationRow, error)
	ReviewTranslation(ctx context.Context, arg dbgen.ReviewTranslationParams) (int64, error)
}

// Handler serves TranslationService. cmd/api lets only admins and
// translators in (auth.TranslatorsOnly); caller() checks again, and every
// RPC checks the language, so a wiring mistake fails closed.
type Handler struct {
	Q            Store
	Catalog      *uistrings.Catalog
	Translations *translate.Service
	// GitHub is nil without GITHUB_TOKEN: UI strings can be read, not saved.
	// Base is the branch pull requests target.
	GitHub Repo
	Base   string
	// Drafter is nil without GOOGLE_GEMINI_API_KEY: no suggestions.
	Drafter Drafter
	Now     func() time.Time // nil = time.Now

	// One save at a time: two saves for a language would otherwise race to
	// open its pull request, or to commit on top of the same tip.
	mu sync.Mutex
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func caller(ctx context.Context) (auth.Caller, error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return c, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	if !c.Admin && len(c.TranslatorLangs) == 0 {
		return c, connect.NewError(connect.CodePermissionDenied, errors.New("translators only"))
	}
	return c, nil
}

// forLang is caller plus the right to edit lang.
func forLang(ctx context.Context, lang string) (auth.Caller, error) {
	c, err := caller(ctx)
	if err != nil {
		return c, err
	}
	if !i18n.ValidDisplay(lang) {
		return c, invalid("lang: an ISO 639-1 code such as \"ja\", or zh-Hant")
	}
	if !c.Translates(lang) {
		return c, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you don't translate %q", lang))
	}
	return c, nil
}

func invalid(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}

func githubError(err error) error { return connect.NewError(connect.CodeUnavailable, err) }

func (h *Handler) GetTranslator(ctx context.Context, _ *connect.Request[isshov1.GetTranslatorRequest]) (*connect.Response[isshov1.GetTranslatorResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.GetTranslatorResponse{
		UserId: c.UserID.String(), Admin: c.Admin, Languages: slices.Sorted(slices.Values(c.TranslatorLangs)), CanPropose: h.GitHub != nil,
	}), nil
}

// UI strings.

func humanPath(lang string) string { return uistrings.HumanDir + "/" + lang + ".json" }

// openPRs maps each language to its editor pull request.
func (h *Handler) openPRs(ctx context.Context) (map[string]github.PR, error) {
	out := map[string]github.PR{}
	if h.GitHub == nil {
		return out, nil
	}
	prs, err := h.GitHub.OpenPRs(ctx, BranchPrefix)
	if err != nil {
		return nil, githubError(err)
	}
	for _, pr := range prs {
		// <lang>-<unix time>; the language may contain a dash (zh-Hant).
		rest := strings.TrimPrefix(pr.Branch, BranchPrefix)
		if i := strings.LastIndex(rest, "-"); i > 0 {
			lang := rest[:i]
			if _, seen := out[lang]; !seen {
				out[lang] = pr
			}
		}
	}
	return out, nil
}

// branchVersions reads lang's human versions on branch; none when the file
// isn't there yet.
func (h *Handler) branchVersions(ctx context.Context, lang, branch string) (map[string]uistrings.Human, error) {
	raw, _, err := h.GitHub.File(ctx, humanPath(lang), branch)
	if errors.Is(err, github.ErrNotFound) {
		return map[string]uistrings.Human{}, nil
	}
	if err != nil {
		return nil, githubError(err)
	}
	var m map[string]uistrings.Human
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s on %s: %w", humanPath(lang), branch, err)
	}
	if m == nil {
		m = map[string]uistrings.Human{}
	}
	return m, nil
}

func prToProto(pr *github.PR) *isshov1.TranslationPullRequest {
	if pr == nil {
		return nil
	}
	return &isshov1.TranslationPullRequest{Number: int32(pr.Number), Url: pr.URL}
}

func (h *Handler) ListAppLanguages(ctx context.Context, _ *connect.Request[isshov1.ListAppLanguagesRequest]) (*connect.Response[isshov1.ListAppLanguagesResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	prs, err := h.openPRs(ctx)
	if err != nil {
		return nil, err
	}
	// Admins see every language there's something for: human versions,
	// an open pull request or machine translations people have asked for.
	langs := map[string]bool{}
	for _, l := range c.TranslatorLangs {
		langs[l] = true
	}
	if c.Admin {
		for _, l := range h.Catalog.HumanLanguages() {
			langs[l] = true
		}
		for l := range prs {
			langs[l] = true
		}
		rows, err := h.Q.ListTranslationLanguages(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			langs[r.Lang] = true
		}
		langs[i18n.Authoring] = true
	}
	out := &isshov1.ListAppLanguagesResponse{}
	for _, l := range slices.Sorted(maps.Keys(langs)) {
		if !i18n.ValidDisplay(l) || !c.Translates(l) {
			continue
		}
		al := &isshov1.AppLanguage{Lang: l, Total: int32(len(h.Catalog.Keys()))}
		for key, v := range h.Catalog.HumanVersions(l) {
			if _, ok := h.Catalog.English(key); !ok {
				continue
			}
			if h.Catalog.Current(key, v) {
				al.Human++
			} else {
				al.Stale++
			}
		}
		if pr, ok := prs[l]; ok {
			al.PullRequest = prToProto(&pr)
		}
		out.Languages = append(out.Languages, al)
	}
	return connect.NewResponse(out), nil
}

// appString builds one row: main's human version (as followed), the open
// pull request's (branch, nil without one) and the machine translation.
func (h *Handler) appString(lang, key, machine string, branch map[string]uistrings.Human) *isshov1.AppString {
	english, _ := h.Catalog.English(key)
	s := &isshov1.AppString{Key: key, English: english, Note: h.Catalog.Note(key), Machine: machine}
	if v, ok := h.Catalog.HumanVersions(i18n.Authoring)[key]; ok && h.Catalog.Current(key, v) {
		s.EnglishVoice = v.Text
	}
	main, onMain := h.Catalog.HumanVersions(lang)[key]
	if onMain {
		s.Human, s.HumanStale = main.Text, !h.Catalog.Current(key, main)
	}
	if branch != nil {
		pr, inPR := branch[key]
		if inPR != onMain || pr != main {
			s.Pending = true
			if inPR {
				s.PendingText = pr.Text
			}
		}
	}
	return s
}

func (h *Handler) ListAppStrings(ctx context.Context, req *connect.Request[isshov1.ListAppStringsRequest]) (*connect.Response[isshov1.ListAppStringsResponse], error) {
	lang := req.Msg.Lang
	if _, err := forLang(ctx, lang); err != nil {
		return nil, err
	}
	machine, complete := map[string]string{}, true
	if lang != i18n.Authoring {
		machine, complete = h.Catalog.Machine(ctx, h.Translations, lang)
	}
	prs, err := h.openPRs(ctx)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListAppStringsResponse{Complete: complete}
	var branch map[string]uistrings.Human
	if pr, ok := prs[lang]; ok {
		out.PullRequest = prToProto(&pr)
		if branch, err = h.branchVersions(ctx, lang, pr.Branch); err != nil {
			return nil, err
		}
	}
	for _, key := range h.Catalog.Keys() {
		out.Strings = append(out.Strings, h.appString(lang, key, machine[key], branch))
	}
	return connect.NewResponse(out), nil
}

func languageName(lang string) string {
	if n := display.English.Languages().Name(language.Make(lang)); n != "" {
		return n
	}
	return lang
}

// change commits edit (which changes a language's human versions; false
// when there's nothing to change) to lang's open pull request, opening one
// on a new branch when there's none. It retries once if the branch moved.
func (h *Handler) change(ctx context.Context, lang, message string, edit func(map[string]uistrings.Human) bool) (*github.PR, map[string]uistrings.Human, error) {
	if h.GitHub == nil {
		return nil, nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("saving translations needs GITHUB_TOKEN on the API"))
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for attempt := 0; ; attempt++ {
		prs, err := h.openPRs(ctx)
		if err != nil {
			return nil, nil, err
		}
		// Without a pull request, start from the base branch's file and only
		// branch off once there's something to commit.
		pr, open := prs[lang]
		from := h.Base
		if open {
			from = pr.Branch
		}
		versions, err := h.branchVersions(ctx, lang, from)
		if err != nil {
			return nil, nil, err
		}
		if !edit(versions) {
			if !open {
				return nil, nil, nil
			}
			return &pr, versions, nil
		}
		branch := pr.Branch
		if !open {
			branch = fmt.Sprintf("%s%s-%d", BranchPrefix, lang, h.now().Unix())
			if err := h.GitHub.CreateBranch(ctx, branch); err != nil {
				return nil, nil, githubError(err)
			}
		}
		content := uistrings.EncodeHuman(versions)
		err = h.GitHub.CommitFiles(ctx, branch, map[string][]byte{
			humanPath(lang): content,
			uistrings.HumanCopyDir + "/" + lang + ".json": content,
		}, message)
		if errors.Is(err, github.ErrConflict) && attempt == 0 {
			continue
		}
		if err != nil {
			return nil, nil, githubError(err)
		}
		if !open {
			name := languageName(lang)
			// The repository is public: the pull request says what changed,
			// never who changed it.
			body := fmt.Sprintf("Human versions of the app's %s text, from the admin SPA's translation editor. Each commit is one string.\n\n"+
				"**After merging:** the API serves them within a few minutes, without a deploy, and phones pick them up at their next hourly check. "+
				"English ones reach the app with its next build (English is bundled).\n", name)
			if pr, err = h.GitHub.OpenPR(ctx, branch, fmt.Sprintf("feat(data): update the %s translations", name), body); err != nil {
				return nil, nil, githubError(err)
			}
		}
		return &pr, versions, nil
	}
}

func (h *Handler) SaveAppString(ctx context.Context, req *connect.Request[isshov1.SaveAppStringRequest]) (*connect.Response[isshov1.SaveAppStringResponse], error) {
	lang, key := req.Msg.Lang, req.Msg.Key
	if _, err := forLang(ctx, lang); err != nil {
		return nil, err
	}
	english, ok := h.Catalog.English(key)
	if !ok {
		return nil, invalid("key: no such UI string")
	}
	text := strings.TrimSpace(req.Msg.Text)
	if text == "" {
		return nil, invalid("text: required")
	}
	// A livelier English identical to the plain one would only duplicate it.
	if lang == i18n.Authoring && text == english {
		return nil, invalid("text: the same as the plain English; remove the livelier version instead")
	}
	// A lost placeholder would never be shown (uistrings.Catalog.Current).
	if !uistrings.SamePlaceholders(english, text) {
		return nil, invalid("text: keep every {placeholder} of the English text, unchanged")
	}
	v := uistrings.Human{Text: text, Source: uistrings.SourceHash(english)}
	pr, branch, err := h.change(ctx, lang, fmt.Sprintf("feat(data): set the %s text of %s", lang, key), func(m map[string]uistrings.Human) bool {
		if m[key] == v {
			return false
		}
		m[key] = v
		return true
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SaveAppStringResponse{
		AppString: h.appString(lang, key, h.machineText(ctx, lang, key), branch), PullRequest: prToProto(pr),
	}), nil
}

func (h *Handler) RemoveAppString(ctx context.Context, req *connect.Request[isshov1.RemoveAppStringRequest]) (*connect.Response[isshov1.RemoveAppStringResponse], error) {
	lang, key := req.Msg.Lang, req.Msg.Key
	if _, err := forLang(ctx, lang); err != nil {
		return nil, err
	}
	if _, ok := h.Catalog.English(key); !ok {
		return nil, invalid("key: no such UI string")
	}
	pr, branch, err := h.change(ctx, lang, fmt.Sprintf("feat(data): remove the %s human version of %s", lang, key), func(m map[string]uistrings.Human) bool {
		if _, ok := m[key]; !ok {
			return false
		}
		delete(m, key)
		return true
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.RemoveAppStringResponse{
		AppString: h.appString(lang, key, h.machineText(ctx, lang, key), branch), PullRequest: prToProto(pr),
	}), nil
}

func (h *Handler) DraftAppString(ctx context.Context, req *connect.Request[isshov1.DraftAppStringRequest]) (*connect.Response[isshov1.DraftAppStringResponse], error) {
	lang, key := req.Msg.Lang, req.Msg.Key
	if _, err := forLang(ctx, lang); err != nil {
		return nil, err
	}
	english, ok := h.Catalog.English(key)
	if !ok {
		return nil, invalid("key: no such UI string")
	}
	if h.Drafter == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("suggestions need GOOGLE_GEMINI_API_KEY on the API"))
	}
	d := translate.Draft{English: english, Context: h.Catalog.Note(key), Current: strings.TrimSpace(req.Msg.Current)}
	if v, ok := h.Catalog.HumanVersions(i18n.Authoring)[key]; ok && h.Catalog.Current(key, v) {
		d.Voice = v.Text
	}
	if d.Current == "" {
		if v, ok := h.Catalog.HumanVersions(lang)[key]; ok && h.Catalog.Current(key, v) {
			d.Current = v.Text
		} else if lang != i18n.Authoring {
			d.Current = h.machineText(ctx, lang, key)
		}
	}
	if lang == i18n.Authoring {
		d.Voice = "" // the draft is the livelier English itself
	}
	text, err := h.Drafter.Draft(ctx, lang, d)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	// A draft that lost a placeholder couldn't be saved anyway.
	if !uistrings.SamePlaceholders(english, text) {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("the suggestion changed a {placeholder}; ask again"))
	}
	return connect.NewResponse(&isshov1.DraftAppStringResponse{Text: text}), nil
}

// machineText is one string's machine translation, normally cached
// already (ListAppStrings asked for it).
func (h *Handler) machineText(ctx context.Context, lang, key string) string {
	if lang == i18n.Authoring {
		return ""
	}
	english, _ := h.Catalog.English(key)
	raw, _ := json.Marshal(map[string]string{i18n.Authoring: english})
	return h.Translations.Resolve(ctx, lang, []translate.Text{{Raw: raw, Context: h.Catalog.Context(key)}})[0]
}

// Catalogue text.

func (h *Handler) ListCatalogLanguages(ctx context.Context, _ *connect.Request[isshov1.ListCatalogLanguagesRequest]) (*connect.Response[isshov1.ListCatalogLanguagesResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := h.Q.ListCatalogLanguages(ctx, uistrings.ContextPrefix)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListCatalogLanguagesResponse{}
	for _, r := range rows {
		if c.Translates(r.Lang) {
			out.Languages = append(out.Languages, &isshov1.TranslationLanguage{Lang: r.Lang, Total: r.Total, Unreviewed: r.Unreviewed})
		}
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ListCatalogTranslations(ctx context.Context, req *connect.Request[isshov1.ListCatalogTranslationsRequest]) (*connect.Response[isshov1.ListCatalogTranslationsResponse], error) {
	if _, err := forLang(ctx, req.Msg.Lang); err != nil {
		return nil, err
	}
	if req.Msg.Offset < 0 {
		return nil, invalid("offset: not negative")
	}
	rows, err := h.Q.ListCatalogTranslations(ctx, dbgen.ListCatalogTranslationsParams{
		Lang: req.Msg.Lang, UiContext: uistrings.ContextPrefix, UnreviewedOnly: req.Msg.UnreviewedOnly,
		Search: strings.TrimSpace(req.Msg.Search), SkipRows: req.Msg.Offset, MaxRows: catalogPage,
	})
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListCatalogTranslationsResponse{Translations: make([]*isshov1.Translation, len(rows))}
	for i, r := range rows {
		out.Translations[i] = translationToProto(dbgen.GetTranslationRow(r))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ReviewCatalogTranslation(ctx context.Context, req *connect.Request[isshov1.ReviewCatalogTranslationRequest]) (*connect.Response[isshov1.ReviewCatalogTranslationResponse], error) {
	c, err := forLang(ctx, req.Msg.Lang)
	if err != nil {
		return nil, err
	}
	key := dbgen.GetTranslationParams{SourceHash: req.Msg.SourceHash, Lang: req.Msg.Lang}
	text := strings.TrimSpace(req.Msg.Text)
	if text == "" {
		return nil, invalid("text: required")
	}
	t, err := h.Q.GetTranslation(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such translation"))
	}
	if err != nil {
		return nil, err
	}
	// UI strings are edited as human versions (SaveAppString): a correction
	// here would be hidden by them and lost when machines redo the cache.
	if strings.HasPrefix(t.Context, uistrings.ContextPrefix) {
		return nil, invalid("this is a UI string: edit it with SaveAppString")
	}
	if !uistrings.SamePlaceholders(t.Source, text) {
		return nil, invalid("text: keep every {placeholder} of the English text, unchanged")
	}
	if _, err := h.Q.ReviewTranslation(ctx, dbgen.ReviewTranslationParams{
		SourceHash: key.SourceHash, Lang: key.Lang, Text: text,
		ReviewedBy: pgtype.UUID{Bytes: c.UserID, Valid: true}, At: pgtype.Timestamptz{Time: h.now(), Valid: true},
	}); err != nil {
		return nil, err
	}
	if t, err = h.Q.GetTranslation(ctx, key); err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.ReviewCatalogTranslationResponse{Translation: translationToProto(t)}), nil
}

func translationToProto(t dbgen.GetTranslationRow) *isshov1.Translation {
	out := &isshov1.Translation{
		SourceHash: t.SourceHash, Lang: t.Lang, Source: t.Source, Context: t.Context,
		Text: t.Text, Model: t.Model, Reviewed: t.Reviewed,
	}
	if t.ReviewedAt.Valid {
		out.ReviewedAt = timestamppb.New(t.ReviewedAt.Time)
	}
	if t.CreatedAt.Valid {
		out.CreatedAt = timestamppb.New(t.CreatedAt.Time)
	}
	return out
}
