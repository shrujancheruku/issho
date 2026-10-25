import { ConnectError, type AppString, type ListAppStringsResponse, type Translation, type TranslationPullRequest } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { placeholderProblems } from '../format';
import { api } from '../lib';

const pageSize = 100; // TranslationService.ListCatalogTranslations returns pages of 100

/**
 * The translation editor, for admins and translators (their languages only).
 * The app's own text is saved as human versions through a pull request per
 * language; catalogue text (activity and city names) is corrected in the
 * database, as before.
 */
export function Translations() {
  const [tab, setTab] = useState<'app' | 'catalogue'>('app');
  return (
    <section>
      <div className="toolbar">
        <h1>Translations</h1>
        <div className="tabs">
          <button className={tab === 'app' ? 'tab active' : 'tab'} onClick={() => setTab('app')}>App text</button>
          <button className={tab === 'catalogue' ? 'tab active' : 'tab'} onClick={() => setTab('catalogue')}>Activities, places and emails</button>
        </div>
      </div>
      {tab === 'app' ? <AppText /> : <Catalogue />}
    </section>
  );
}

/**
 * Lines a textarea needs for text: Chinese, Japanese and Korean characters
 * take about two Latin widths, so counting characters alone clipped them.
 */
function rowsFor(text: string) {
  let width = 0;
  for (const ch of text) width += (ch.codePointAt(0) ?? 0) >= 0x2e80 ? 2 : 1;
  return Math.min(8, Math.max(1, Math.ceil(width / 52)));
}

type Filter = 'check' | 'stale' | 'pending' | 'all';
const filters: { id: Filter; label: string }[] = [
  { id: 'check', label: 'To check' },
  { id: 'stale', label: 'English changed' },
  { id: 'pending', label: 'In pull request' },
  { id: 'all', label: 'All' },
];

/** What the app shows for s today: a current human version, else the machine's (or the English). */
function shown(s: AppString, lang: string) {
  if (s.human && !s.humanStale) return s.human;
  return lang === 'en' ? s.english : s.machine;
}

function matches(s: AppString, filter: Filter, query: string) {
  const needs = filter === 'all'
    || (filter === 'check' && !s.pending && (!s.human || s.humanStale))
    || (filter === 'stale' && s.humanStale && !s.pending)
    || (filter === 'pending' && s.pending);
  if (!needs) return false;
  if (!query) return true;
  const q = query.toLowerCase();
  return [s.key, s.english, s.englishVoice, s.machine, s.human, s.pendingText].some((t) => t.toLowerCase().includes(q));
}

function AppText() {
  const queryClient = useQueryClient();
  const me = useQuery({ queryKey: ['translator'], queryFn: () => api.translation.getTranslator({}) });
  const languages = useQuery({ queryKey: ['app-languages'], queryFn: () => api.translation.listAppLanguages({}) });
  const [lang, setLang] = useState('');
  const [filter, setFilter] = useState<Filter>('check');
  const [search, setSearch] = useState('');

  // Opening a language starts its machine translations (a cost), so start
  // where there's work: an open pull request, then human versions, then
  // English, which needs no machine.
  useEffect(() => {
    const all = languages.data?.languages ?? [];
    const first = all.find((l) => l.pullRequest) ?? all.find((l) => l.lang !== 'en' && l.human + l.stale > 0)
      ?? all.find((l) => l.lang === 'en') ?? all[0];
    if (!lang && first) setLang(first.lang);
  }, [lang, languages.data]);

  const list = useQuery({
    queryKey: ['app-strings', lang],
    queryFn: () => api.translation.listAppStrings({ lang }),
    enabled: lang !== '',
    // The first look at a language starts its machine translations.
    refetchInterval: (q) => (q.state.data && !q.state.data.complete ? 5000 : false),
  });
  const [pr, setPr] = useState<TranslationPullRequest | undefined>();
  useEffect(() => setPr(list.data?.pullRequest), [list.data]);

  const onSaved = (saved?: AppString, next?: TranslationPullRequest) => {
    if (next) setPr(next);
    // The filters read the list: put the saved row in it, so it moves to
    // "In pull request" without fetching every string again.
    if (saved) {
      queryClient.setQueryData<ListAppStringsResponse>(['app-strings', lang], (old) =>
        old && { ...old, strings: old.strings.map((s) => (s.key === saved.key ? saved : s)) });
    }
    queryClient.invalidateQueries({ queryKey: ['app-languages'] });
  };

  const rows = list.data?.strings.filter((s) => matches(s, filter, search.trim())) ?? [];
  const canPropose = me.data?.canPropose ?? false;
  return (
    <>
      <div className="toolbar">
        <select value={lang} onChange={(e) => setLang(e.target.value)}>
          {languages.data?.languages.map((l) => (
            <option key={l.lang} value={l.lang}>
              {l.lang} · {l.human} of {l.total} checked{l.stale ? `, ${l.stale} with changed English` : ''}{l.pullRequest ? ' · pull request open' : ''}
            </option>
          ))}
        </select>
        <div className="tabs">
          {filters.map((f) => (
            <button key={f.id} className={f.id === filter ? 'tab active' : 'tab'} onClick={() => setFilter(f.id)}>{f.label}</button>
          ))}
        </div>
        <input type="search" placeholder="Search" value={search} onChange={(e) => setSearch(e.target.value)} />
      </div>
      <p className="muted hint">
        The app's own text. Machines translate the plain English; a person's version replaces it for that string.
        {' '}<strong>Looks right</strong> keeps the machine's text as checked; <strong>Save</strong> keeps yours. Keep every
        {' {placeholder}'} as it is, and write what someone using the app would naturally read.
      </p>
      {!canPropose && me.data && <p className="error">Saving needs GITHUB_TOKEN on the API. You can read, not save.</p>}
      {pr
        ? <p className="hint">Your changes go to <a href={pr.url} target="_blank" rel="noreferrer">pull request #{pr.number}</a>. Once it's merged, the app shows them within the hour.</p>
        : lang && <p className="muted hint">Your first change opens a pull request for this language; the next ones join it.</p>}
      {(languages.error || list.error) && <p className="error">{ConnectError.from(languages.error ?? list.error).message}</p>}
      {languages.data?.languages.length === 0 && <p className="empty">No languages to translate.</p>}
      {list.isFetching && !list.data && <p className="muted">Loading…</p>}
      {list.data && !list.data.complete && <p className="muted">Machine translations are still being made…</p>}
      {list.data && rows.length === 0 && <p className="empty">Nothing here.</p>}
      {rows.map((s) => <AppStringRow key={`${lang}/${s.key}`} lang={lang} initial={s} canPropose={canPropose} onSaved={onSaved} />)}
    </>
  );
}

function AppStringRow({ lang, initial, canPropose, onSaved }: {
  lang: string; initial: AppString; canPropose: boolean; onSaved: (saved?: AppString, pr?: TranslationPullRequest) => void;
}) {
  const [s, setS] = useState(initial);
  // Machine translations can arrive after the row first renders.
  useEffect(() => setS((cur) => (cur.machine ? cur : { ...cur, machine: initial.machine })), [initial.machine]);
  const current = s.pending ? (s.pendingText || (lang === 'en' ? s.english : s.machine)) : shown(s, lang);
  const [text, setText] = useState(current);
  useEffect(() => setText((t) => t || current), [current]);
  const problems = placeholderProblems(s.english, text);
  const hasHuman = s.pending ? s.pendingText !== '' : s.human !== '';

  const done = (res: { appString?: AppString; pullRequest?: TranslationPullRequest }) => {
    if (res.appString) {
      setS(res.appString);
      setText(res.appString.pending ? (res.appString.pendingText || res.appString.machine || res.appString.english) : shown(res.appString, lang));
    }
    onSaved(res.appString, res.pullRequest);
  };
  const save = useMutation({ mutationFn: (t: string) => api.translation.saveAppString({ lang, key: s.key, text: t }), onSuccess: done });
  const remove = useMutation({ mutationFn: () => api.translation.removeAppString({ lang, key: s.key }), onSuccess: done });
  const busy = save.isPending || remove.isPending;
  const edited = text.trim() !== current.trim();
  const isMachine = lang !== 'en' && text.trim() === s.machine.trim();
  // A machine text that's already kept (checked) needs nothing more.
  const alreadyKept = hasHuman && !edited && !(s.humanStale && !s.pending);

  let status = lang === 'en' ? 'Plain English' : 'Machine translation';
  if (s.pending) status = s.pendingText ? 'In the pull request' : 'Removed in the pull request';
  else if (s.humanStale) status = 'The English changed since this was written: check it';
  else if (s.human) status = 'Checked';
  const error = save.error ?? remove.error;

  return (
    <article className={!s.pending && s.human && !s.humanStale ? 'card translation reviewed' : s.pending ? 'card translation pending' : 'card translation'}>
      <div className="source">
        <p>{s.english}</p>
        {s.englishVoice && lang !== 'en' && <p className="meta">Shown in English as: “{s.englishVoice}”</p>}
        {s.note && <p className="meta">{s.note}</p>}
        <p className="meta"><code>{s.key}</code></p>
      </div>
      <div className="target">
        <p className="meta">{status}</p>
        <textarea rows={rowsFor(text)} lang={lang} dir="auto" value={text} onChange={(e) => setText(e.target.value)} />
        {lang !== 'en' && s.machine && !isMachine && <p className="meta">Machine: {s.machine}</p>}
        {s.humanStale && s.human && <p className="meta">Written for the old English: {s.human}</p>}
        <div className="row">
          {/* English has no machine text to approve: only a different wording is worth saving. */}
          {edited || (!alreadyKept && lang !== 'en')
            ? (
              <button className="small" disabled={!canPropose || busy || problems.length > 0 || text.trim() === ''} onClick={() => save.mutate(text)}>
                {isMachine ? 'Looks right' : 'Save'}
              </button>
            )
            : null}
          {hasHuman && (
            <button className="small secondary" disabled={!canPropose || busy} onClick={() => remove.mutate()}>
              {lang === 'en' ? 'Use the plain English' : 'Use the machine text'}
            </button>
          )}
          {edited && <button className="link" onClick={() => setText(current)}>Undo</button>}
          {problems.length > 0 && <span className="error">{problems.join(', ')}</span>}
          {error && <span className="error">{ConnectError.from(error).message}</span>}
        </div>
      </div>
    </article>
  );
}

/** Activity and city names and the sign-in email: machine translations corrected in the database. */
function Catalogue() {
  const languages = useQuery({ queryKey: ['catalogue-languages'], queryFn: () => api.translation.listCatalogLanguages({}) });
  const [lang, setLang] = useState('');
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState(''); // search, applied after a pause in typing
  const [unreviewedOnly, setUnreviewedOnly] = useState(true);
  const [offset, setOffset] = useState(0);

  useEffect(() => {
    if (!lang && languages.data?.languages.length) setLang(languages.data.languages[0].lang);
  }, [lang, languages.data]);
  useEffect(() => {
    const t = setTimeout(() => { setQuery(search.trim()); setOffset(0); }, 300);
    return () => clearTimeout(t);
  }, [search]);

  const list = useQuery({
    queryKey: ['catalogue-translations', lang, unreviewedOnly, query, offset],
    queryFn: () => api.translation.listCatalogTranslations({ lang, unreviewedOnly, search: query, offset }),
    enabled: lang !== '',
  });

  return (
    <>
      <div className="toolbar">
        <select value={lang} onChange={(e) => { setLang(e.target.value); setOffset(0); }}>
          {languages.data?.languages.map((l) => (
            <option key={l.lang} value={l.lang}>{l.lang} · {l.unreviewed} to check of {l.total}</option>
          ))}
        </select>
        <input type="search" placeholder="Search English or translation" value={search} onChange={(e) => setSearch(e.target.value)} />
        <label className="check">
          <input type="checkbox" checked={unreviewedOnly} onChange={(e) => { setUnreviewedOnly(e.target.checked); setOffset(0); }} />
          Unchecked only
        </label>
      </div>
      <p className="muted hint">
        Names of activities and cities, and the sign-in email's sentences (used the next time the emails are built),
        machine-translated and shared by everyone who reads that language. Saving marks one as checked, and a checked
        translation is never replaced by a machine one.
      </p>
      {languages.data?.languages.length === 0 && <p className="empty">No machine translations yet.</p>}
      {(languages.error || list.error) && <p className="error">{ConnectError.from(languages.error ?? list.error).message}</p>}
      {list.isFetching && !list.data && <p className="muted">Loading…</p>}
      {list.data?.translations.length === 0 && <p className="empty">Nothing here.</p>}
      {list.data?.translations.map((t) => <CatalogueRow key={`${t.sourceHash}/${t.lang}`} translation={t} />)}
      {list.data && (offset > 0 || list.data.translations.length === pageSize) && (
        <div className="row pager">
          <button className="secondary" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - pageSize))}>Previous</button>
          <span className="muted">{offset + 1}–{offset + list.data.translations.length}</span>
          <button className="secondary" disabled={list.data.translations.length < pageSize} onClick={() => setOffset(offset + pageSize)}>Next</button>
        </div>
      )}
    </>
  );
}

function CatalogueRow({ translation }: { translation: Translation }) {
  const queryClient = useQueryClient();
  const [t, setT] = useState(translation);
  const [text, setText] = useState(translation.text);
  const problems = placeholderProblems(t.source, text);
  const save = useMutation({
    mutationFn: () => api.translation.reviewCatalogTranslation({ sourceHash: t.sourceHash, lang: t.lang, text }),
    onSuccess: (res) => {
      if (res.translation) { setT(res.translation); setText(res.translation.text); }
      queryClient.invalidateQueries({ queryKey: ['catalogue-languages'] });
    },
  });

  return (
    <article className={t.reviewed ? 'card translation reviewed' : 'card translation'}>
      <div className="source">
        <p>{t.source}</p>
        <p className="meta">{t.context} · {t.reviewed ? 'checked' : t.model}</p>
      </div>
      <div className="target">
        <textarea rows={rowsFor(text)} lang={t.lang} dir="auto" value={text} onChange={(e) => setText(e.target.value)} />
        <div className="row">
          <button className="small" disabled={save.isPending || problems.length > 0 || text.trim() === '' || (t.reviewed && text === t.text)} onClick={() => save.mutate()}>
            {text === t.text ? 'Looks right' : 'Save correction'}
          </button>
          {problems.length > 0 && <span className="error">{problems.join(', ')}</span>}
          {save.error && <span className="error">{ConnectError.from(save.error).message}</span>}
        </div>
      </div>
    </article>
  );
}
