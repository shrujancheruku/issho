package uistrings

// The UI-string editor's domain: human versions are edited in the
// repository, like city packs (owner's decision). Every save is a commit to
// one rolling pull request per language, which the owner merges; the API
// then serves it without a deploy (Catalog.Follow). translations.Handler
// checks the caller and the language and maps protos; the rules and the
// GitHub calls are here (the repoedit follow-up moves the calls).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

// BranchPrefix marks the editor's branches: translations/<lang>-<unix time>.
const BranchPrefix = "translations/"

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

// Editor edits the human versions of the UI strings. Callers have already
// checked that the caller may edit the language.
type Editor struct {
	Catalog      *Catalog
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

func (e *Editor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// CanPropose reports whether saving works (a GitHub token is set).
func (e *Editor) CanPropose() bool { return e.GitHub != nil }

// githubError keeps GitHub's own message off the wire (logs and Sentry only).
func githubError(err error) error { return errs.Upstream("github", err) }

func humanPath(lang string) string { return HumanDir + "/" + lang + ".json" }

// Row is one UI string as the editor shows it: main's human version (as
// followed), the open pull request's and the machine translation.
type Row struct {
	Key, English, Note, Machine string
	// EnglishVoice is the owner's livelier English, when current.
	EnglishVoice string
	Human        string
	HumanStale   bool
	// Pending is true when the open pull request changes the string;
	// PendingText is its version (empty when it removes one).
	Pending     bool
	PendingText string
}

// OpenPRs maps each language to its editor pull request.
func (e *Editor) OpenPRs(ctx context.Context) (map[string]github.PR, error) {
	out := map[string]github.PR{}
	if e.GitHub == nil {
		return out, nil
	}
	prs, err := e.GitHub.OpenPRs(ctx, BranchPrefix)
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
func (e *Editor) branchVersions(ctx context.Context, lang, branch string) (map[string]Human, error) {
	raw, _, err := e.GitHub.File(ctx, humanPath(lang), branch)
	if errors.Is(err, github.ErrNotFound) {
		return map[string]Human{}, nil
	}
	if err != nil {
		return nil, githubError(err)
	}
	var m map[string]Human
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s on %s: %w", humanPath(lang), branch, err)
	}
	if m == nil {
		m = map[string]Human{}
	}
	return m, nil
}

// Progress counts lang's human versions on main: current ones and stale
// ones (written for English that has since changed), and the strings in all.
func (e *Editor) Progress(lang string) (total, human, stale int) {
	for key, v := range e.Catalog.HumanVersions(lang) {
		if _, ok := e.Catalog.English(key); !ok {
			continue
		}
		if e.Catalog.Current(key, v) {
			human++
		} else {
			stale++
		}
	}
	return len(e.Catalog.Keys()), human, stale
}

// row builds one row; branch is the open pull request's versions (nil
// without one).
func (e *Editor) row(lang, key, machine string, branch map[string]Human) Row {
	english, _ := e.Catalog.English(key)
	s := Row{Key: key, English: english, Note: e.Catalog.Note(key), Machine: machine}
	if v, ok := e.Catalog.HumanVersions(i18n.Authoring)[key]; ok && e.Catalog.Current(key, v) {
		s.EnglishVoice = v.Text
	}
	main, onMain := e.Catalog.HumanVersions(lang)[key]
	if onMain {
		s.Human, s.HumanStale = main.Text, !e.Catalog.Current(key, main)
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

// Listing is every UI string in a language.
type Listing struct {
	Rows []Row
	// Complete is false while machine translations are still to come.
	Complete bool
	// PR is the language's open pull request, if it has one.
	PR *github.PR
}

// List returns every UI string in lang with its human, pending and machine
// versions.
func (e *Editor) List(ctx context.Context, lang string) (Listing, error) {
	machine, complete := map[string]string{}, true
	if lang != i18n.Authoring {
		machine, complete = e.Catalog.Machine(ctx, e.Translations, lang)
	}
	prs, err := e.OpenPRs(ctx)
	if err != nil {
		return Listing{}, err
	}
	out := Listing{Complete: complete}
	var branch map[string]Human
	if pr, ok := prs[lang]; ok {
		out.PR = &pr
		if branch, err = e.branchVersions(ctx, lang, pr.Branch); err != nil {
			return Listing{}, err
		}
	}
	for _, key := range e.Catalog.Keys() {
		out.Rows = append(out.Rows, e.row(lang, key, machine[key], branch))
	}
	return out, nil
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
func (e *Editor) change(ctx context.Context, lang, message string, edit func(map[string]Human) bool) (*github.PR, map[string]Human, error) {
	if e.GitHub == nil {
		return nil, nil, errs.Precondition(isshov1.ErrorReason_ERROR_REASON_UNSPECIFIED, "saving translations needs GITHUB_TOKEN on the API")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for attempt := 0; ; attempt++ {
		prs, err := e.OpenPRs(ctx)
		if err != nil {
			return nil, nil, err
		}
		// Without a pull request, start from the base branch's file and only
		// branch off once there's something to commit.
		pr, open := prs[lang]
		from := e.Base
		if open {
			from = pr.Branch
		}
		versions, err := e.branchVersions(ctx, lang, from)
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
			branch = fmt.Sprintf("%s%s-%d", BranchPrefix, lang, e.now().Unix())
			if err := e.GitHub.CreateBranch(ctx, branch); err != nil {
				return nil, nil, githubError(err)
			}
		}
		content := EncodeHuman(versions)
		err = e.GitHub.CommitFiles(ctx, branch, map[string][]byte{
			humanPath(lang):                     content,
			HumanCopyDir + "/" + lang + ".json": content,
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
			if pr, err = e.GitHub.OpenPR(ctx, branch, fmt.Sprintf("feat(data): update the %s translations", name), body); err != nil {
				return nil, nil, githubError(err)
			}
		}
		return &pr, versions, nil
	}
}

// Saved is what a save or removal leaves: the string's row afterwards and
// the language's pull request (nil when nothing was changed and none is open).
type Saved struct {
	Row Row
	PR  *github.PR
}

// Save sets lang's human version of key to text.
func (e *Editor) Save(ctx context.Context, lang, key, text string) (Saved, error) {
	english, ok := e.Catalog.English(key)
	if !ok {
		return Saved{}, errs.Invalid("key: no such UI string")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Saved{}, errs.Invalid("text: required")
	}
	// A livelier English identical to the plain one would only duplicate it.
	if lang == i18n.Authoring && text == english {
		return Saved{}, errs.Invalid("text: the same as the plain English; remove the livelier version instead")
	}
	// A lost placeholder would never be shown (Catalog.Current).
	if !SamePlaceholders(english, text) {
		return Saved{}, errs.Invalid("text: keep every {placeholder} of the English text, unchanged")
	}
	v := Human{Text: text, Source: SourceHash(english)}
	pr, branch, err := e.change(ctx, lang, fmt.Sprintf("feat(data): set the %s text of %s", lang, key), func(m map[string]Human) bool {
		if m[key] == v {
			return false
		}
		m[key] = v
		return true
	})
	if err != nil {
		return Saved{}, err
	}
	return Saved{Row: e.row(lang, key, e.machineText(ctx, lang, key), branch), PR: pr}, nil
}

// Remove deletes lang's human version of key.
func (e *Editor) Remove(ctx context.Context, lang, key string) (Saved, error) {
	if _, ok := e.Catalog.English(key); !ok {
		return Saved{}, errs.Invalid("key: no such UI string")
	}
	pr, branch, err := e.change(ctx, lang, fmt.Sprintf("feat(data): remove the %s human version of %s", lang, key), func(m map[string]Human) bool {
		if _, ok := m[key]; !ok {
			return false
		}
		delete(m, key)
		return true
	})
	if err != nil {
		return Saved{}, err
	}
	return Saved{Row: e.row(lang, key, e.machineText(ctx, lang, key), branch), PR: pr}, nil
}

// Draft asks the model for a better version of key in lang, starting from
// current (the editor's text; empty = what is shown today). Not stored.
func (e *Editor) Draft(ctx context.Context, lang, key, current string) (string, error) {
	english, ok := e.Catalog.English(key)
	if !ok {
		return "", errs.Invalid("key: no such UI string")
	}
	if e.Drafter == nil {
		return "", errs.Precondition(isshov1.ErrorReason_ERROR_REASON_UNSPECIFIED, "suggestions need GOOGLE_GEMINI_API_KEY on the API")
	}
	d := translate.Draft{English: english, Context: e.Catalog.Note(key), Current: strings.TrimSpace(current)}
	if v, ok := e.Catalog.HumanVersions(i18n.Authoring)[key]; ok && e.Catalog.Current(key, v) {
		d.Voice = v.Text
	}
	if d.Current == "" {
		if v, ok := e.Catalog.HumanVersions(lang)[key]; ok && e.Catalog.Current(key, v) {
			d.Current = v.Text
		} else if lang != i18n.Authoring {
			d.Current = e.machineText(ctx, lang, key)
		}
	}
	if lang == i18n.Authoring {
		d.Voice = "" // the draft is the livelier English itself
	}
	text, err := e.Drafter.Draft(ctx, lang, d)
	if errors.Is(err, translate.ErrBudget) {
		return "", errs.UpstreamReason("gemini", isshov1.ErrorReason_ERROR_REASON_TIMED_OUT, "gemini took too long; try again", err)
	}
	if err != nil {
		return "", errs.Upstream("gemini", err)
	}
	// A draft that lost a placeholder couldn't be saved anyway.
	if !SamePlaceholders(english, text) {
		return "", errs.Refused("the suggestion changed a {placeholder}; ask again")
	}
	return text, nil
}

// machineText is one string's machine translation, normally cached
// already (List asked for it).
func (e *Editor) machineText(ctx context.Context, lang, key string) string {
	if lang == i18n.Authoring {
		return ""
	}
	english, _ := e.Catalog.English(key)
	raw, _ := json.Marshal(map[string]string{i18n.Authoring: english})
	return e.Translations.Resolve(ctx, lang, []translate.Text{{Raw: raw, Context: e.Catalog.Context(key)}})[0]
}
