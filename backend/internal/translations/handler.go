// Package translations serves TranslationService: the admin SPA's
// translation editor, for admins and translators.
//
// The handler checks the caller and the language and maps protos; the work
// is done by two services. UI strings are human versions in the repository
// (uistrings.Editor); catalogue text (activity and city names) has no file,
// so its machine translations are corrected in app.translations
// (translate.Reviewer).
package translations

import (
	"context"
	"maps"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/github"
	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
	"github.com/shrujancheruku/issho/backend/internal/platform/paging"
	"github.com/shrujancheruku/issho/backend/internal/translate"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// Handler serves TranslationService. cmd/api lets only admins and
// translators in (auth.TranslatorsOnly); caller() checks again, and every
// RPC checks the language, so a wiring mistake fails closed.
type Handler struct {
	Editor  *uistrings.Editor
	Reviews *translate.Reviewer
}

func caller(ctx context.Context) (auth.Caller, error) {
	c, err := auth.Require(ctx)
	if err != nil {
		return c, err
	}
	if !c.Admin && len(c.TranslatorLangs) == 0 {
		return c, errs.Forbidden("translators only")
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
		return c, errs.Invalid("lang: an ISO 639-1 code such as \"ja\", or zh-Hant")
	}
	if !c.Translates(lang) {
		return c, errs.Forbidden("you don't translate %q", lang)
	}
	return c, nil
}

func (h *Handler) GetTranslator(ctx context.Context, _ *connect.Request[isshov1.GetTranslatorRequest]) (*connect.Response[isshov1.GetTranslatorResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.GetTranslatorResponse{
		UserId: c.UserID.String(), Admin: c.Admin, Languages: slices.Sorted(slices.Values(c.TranslatorLangs)), CanPropose: h.Editor.CanPropose(),
	}), nil
}

// UI strings.

func prToProto(pr *github.PR) *isshov1.TranslationPullRequest {
	if pr == nil {
		return nil
	}
	return &isshov1.TranslationPullRequest{Number: int32(pr.Number), Url: pr.URL}
}

func appString(r uistrings.Row) *isshov1.AppString {
	return &isshov1.AppString{
		Key: r.Key, English: r.English, Note: r.Note, Machine: r.Machine, EnglishVoice: r.EnglishVoice,
		Human: r.Human, HumanStale: r.HumanStale, Pending: r.Pending, PendingText: r.PendingText,
	}
}

func (h *Handler) ListAppLanguages(ctx context.Context, _ *connect.Request[isshov1.ListAppLanguagesRequest]) (*connect.Response[isshov1.ListAppLanguagesResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	prs, err := h.Editor.OpenPRs(ctx)
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
		for _, l := range h.Editor.Catalog.HumanLanguages() {
			langs[l] = true
		}
		for l := range prs {
			langs[l] = true
		}
		rows, err := h.Reviews.CachedLanguages(ctx)
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
		total, human, stale := h.Editor.Progress(l)
		al := &isshov1.AppLanguage{Lang: l, Total: int32(total), Human: int32(human), Stale: int32(stale)}
		if pr, ok := prs[l]; ok {
			al.PullRequest = prToProto(&pr)
		}
		out.Languages = append(out.Languages, al)
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) ListAppStrings(ctx context.Context, req *connect.Request[isshov1.ListAppStringsRequest]) (*connect.Response[isshov1.ListAppStringsResponse], error) {
	if _, err := forLang(ctx, req.Msg.Lang); err != nil {
		return nil, err
	}
	list, err := h.Editor.List(ctx, req.Msg.Lang)
	if err != nil {
		return nil, err
	}
	out := &isshov1.ListAppStringsResponse{Complete: list.Complete, PullRequest: prToProto(list.PR)}
	for _, r := range list.Rows {
		out.Strings = append(out.Strings, appString(r))
	}
	return connect.NewResponse(out), nil
}

func (h *Handler) SaveAppString(ctx context.Context, req *connect.Request[isshov1.SaveAppStringRequest]) (*connect.Response[isshov1.SaveAppStringResponse], error) {
	if _, err := forLang(ctx, req.Msg.Lang); err != nil {
		return nil, err
	}
	saved, err := h.Editor.Save(ctx, req.Msg.Lang, req.Msg.Key, req.Msg.Text)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.SaveAppStringResponse{AppString: appString(saved.Row), PullRequest: prToProto(saved.PR)}), nil
}

func (h *Handler) RemoveAppString(ctx context.Context, req *connect.Request[isshov1.RemoveAppStringRequest]) (*connect.Response[isshov1.RemoveAppStringResponse], error) {
	if _, err := forLang(ctx, req.Msg.Lang); err != nil {
		return nil, err
	}
	saved, err := h.Editor.Remove(ctx, req.Msg.Lang, req.Msg.Key)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.RemoveAppStringResponse{AppString: appString(saved.Row), PullRequest: prToProto(saved.PR)}), nil
}

func (h *Handler) DraftAppString(ctx context.Context, req *connect.Request[isshov1.DraftAppStringRequest]) (*connect.Response[isshov1.DraftAppStringResponse], error) {
	if _, err := forLang(ctx, req.Msg.Lang); err != nil {
		return nil, err
	}
	text, err := h.Editor.Draft(ctx, req.Msg.Lang, req.Msg.Key, req.Msg.Current)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&isshov1.DraftAppStringResponse{Text: text}), nil
}

// Catalogue text.

func (h *Handler) ListCatalogLanguages(ctx context.Context, _ *connect.Request[isshov1.ListCatalogLanguagesRequest]) (*connect.Response[isshov1.ListCatalogLanguagesResponse], error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := h.Reviews.CatalogLanguages(ctx)
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
	size, err := paging.Size(req.Msg.PageSize)
	if err != nil {
		return nil, err
	}
	search := strings.TrimSpace(req.Msg.Search)
	scope := paging.Scope("ListCatalogTranslations", req.Msg.Lang, paging.Bool(req.Msg.UnreviewedOnly), search)
	k, err := paging.Open(req.Msg.PageToken, scope, 4)
	if err != nil {
		return nil, err
	}
	arg := dbgen.ListCatalogTranslationsParams{
		Lang: req.Msg.Lang, UnreviewedOnly: req.Msg.UnreviewedOnly,
		Search: search, MaxRows: size + 1,
	}
	arg.HasAfter, arg.AfterReviewed, arg.AfterSource, arg.AfterContext, arg.AfterHash = k.Present, k.Bool(), k.String(), k.String(), k.String()
	if err := k.Err(); err != nil {
		return nil, err
	}
	rows, err := h.Reviews.CatalogTranslations(ctx, arg)
	if err != nil {
		return nil, err
	}
	rows, more := paging.Trim(rows, size)
	out := &isshov1.ListCatalogTranslationsResponse{Translations: make([]*isshov1.Translation, len(rows))}
	if more {
		last := rows[len(rows)-1]
		out.NextPageToken = paging.Encode(scope, paging.Bool(last.Reviewed), last.Source, last.Context, last.SourceHash)
	}
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
	t, err := h.Reviews.Review(ctx, c.UserID, req.Msg.Lang, req.Msg.SourceHash, req.Msg.Text)
	if err != nil {
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
