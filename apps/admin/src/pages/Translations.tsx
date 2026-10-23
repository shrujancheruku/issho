import { ConnectError, type Translation } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { placeholderProblems } from '../format';
import { api } from '../lib';

const pageSize = 100; // ModerationService.ListTranslations returns pages of 100

export function Translations() {
  const languages = useQuery({ queryKey: ['translation-languages'], queryFn: () => api.moderation.listTranslationLanguages({}) });
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
    queryKey: ['translations', lang, unreviewedOnly, query, offset],
    queryFn: () => api.moderation.listTranslations({ lang, unreviewedOnly, search: query, offset }),
    enabled: lang !== '',
  });

  return (
    <section>
      <div className="toolbar">
        <h1>Translations</h1>
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
        Machine translations, shared by everyone who reads that language. Saving marks one as checked, and a checked
        translation is never replaced by a machine one. Keep every {'{placeholder}'} as it is.
      </p>
      {languages.data?.languages.length === 0 && <p className="empty">No machine translations yet.</p>}
      {(languages.error || list.error) && <p className="error">{ConnectError.from(languages.error ?? list.error).message}</p>}
      {list.isFetching && !list.data && <p className="muted">Loading…</p>}
      {list.data?.translations.length === 0 && <p className="empty">Nothing here.</p>}
      {list.data?.translations.map((t) => <TranslationRow key={`${t.sourceHash}/${t.lang}`} translation={t} />)}
      {list.data && (offset > 0 || list.data.translations.length === pageSize) && (
        <div className="row pager">
          <button className="secondary" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - pageSize))}>Previous</button>
          <span className="muted">{offset + 1}–{offset + list.data.translations.length}</span>
          <button className="secondary" disabled={list.data.translations.length < pageSize} onClick={() => setOffset(offset + pageSize)}>Next</button>
        </div>
      )}
    </section>
  );
}

function TranslationRow({ translation }: { translation: Translation }) {
  const queryClient = useQueryClient();
  const [t, setT] = useState(translation);
  const [text, setText] = useState(translation.text);
  const problems = placeholderProblems(t.source, text);
  const save = useMutation({
    mutationFn: () => api.moderation.reviewTranslation({ sourceHash: t.sourceHash, lang: t.lang, text }),
    onSuccess: (res) => {
      if (res.translation) { setT(res.translation); setText(res.translation.text); }
      // Counts change; the list itself stays put so the row doesn't jump away.
      queryClient.invalidateQueries({ queryKey: ['translation-languages'] });
    },
  });

  return (
    <article className={t.reviewed ? 'card translation reviewed' : 'card translation'}>
      <div className="source">
        <p>{t.source}</p>
        <p className="meta">{t.context} · {t.reviewed ? 'checked' : t.model}</p>
      </div>
      <div className="target">
        <textarea rows={Math.min(6, Math.max(1, Math.ceil(text.length / 60)))} lang={t.lang} dir="auto" value={text} onChange={(e) => setText(e.target.value)} />
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
