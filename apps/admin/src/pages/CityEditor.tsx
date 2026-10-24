import { ConnectError, type CityPack, type CityPackPullRequest, type PackActivity } from '@issho/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { api } from '../lib';

type Area = { id: string; name: Record<string, string>; lat: string; lng: string; existing: boolean };
type Override = { mode: 'default' | 'off' | 'custom'; name: string; months: number[] };
type Draft = {
  id: string; comment: string; name: Record<string, string>; timezone: string; currency: string;
  languages: string; walk: boolean; drive: boolean; areas: Area[]; overrides: Record<string, Override>;
};

// What proposeCityPack takes: plain objects for the messages.
type PackInit = NonNullable<Parameters<typeof api.cityPacks.proposeCityPack>[0]['pack']>;

const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const slug = (s: string) => s.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');

function draftFrom(p: CityPack | undefined): Draft {
  if (!p) {
    return { id: '', comment: '', name: { en: '' }, timezone: '', currency: '', languages: 'en', walk: true, drive: true, areas: [], overrides: {} };
  }
  const overrides: Record<string, Override> = {};
  for (const o of p.activityOverrides) {
    overrides[o.activityId] = { mode: o.disabled ? 'off' : 'custom', name: o.name, months: [...o.months] };
  }
  return {
    id: p.id, comment: p.comment, name: { ...p.name }, timezone: p.timezone, currency: p.currency,
    languages: p.languages.join(', '), walk: p.travelModes.includes('walk'), drive: p.travelModes.includes('drive'),
    areas: p.areas.map((a) => ({ id: a.id, name: { ...a.name }, lat: String(a.lat), lng: String(a.lng), existing: true })),
    overrides,
  };
}

function packFrom(d: Draft): PackInit {
  const languages = d.languages.split(/[\s,]+/).map((l) => l.trim().toLowerCase()).filter(Boolean);
  return {
    id: d.id.trim(), comment: d.comment, name: d.name, timezone: d.timezone, currency: d.currency, languages,
    travelModes: [...(d.walk ? ['walk'] : []), ...(d.drive ? ['drive'] : [])],
    areas: d.areas.map((a) => ({ id: a.id.trim(), name: a.name, lat: Number(a.lat), lng: Number(a.lng) })),
    // Rows left at the catalogue's defaults aren't written to the file. The
    // file's order is kept (new ones go last), so the diff shows only edits.
    activityOverrides: Object.entries(d.overrides)
      .filter(([, o]) => o.mode !== 'default')
      .map(([activityId, o]) => ({ activityId, disabled: o.mode === 'off', name: o.mode === 'custom' ? o.name : '', months: o.mode === 'custom' ? o.months : [] })),
  };
}

/** Edits a city pack (or makes a new one) and proposes it as a GitHub pull request. */
export function CityEditor({ cityId }: { cityId: string }) {
  const isNew = cityId === 'new';
  const res = useQuery({
    queryKey: ['city-pack', cityId],
    queryFn: () => api.cityPacks.getCityPack({ cityId: isNew ? '' : cityId }),
    // A draft must not be replaced by a refetch while it's being edited.
    staleTime: Infinity, refetchOnWindowFocus: false,
  });
  if (res.isPending) return <p className="muted">Loading…</p>;
  if (res.error) return <p className="error">{ConnectError.from(res.error).message}</p>;
  return <Editor key={cityId} isNew={isNew} pack={res.data.pack} activities={res.data.activities} />;
}

function Editor({ isNew, pack, activities }: { isNew: boolean; pack?: CityPack; activities: PackActivity[] }) {
  const [d, setD] = useState<Draft>(() => draftFrom(pack));
  const [note, setNote] = useState('');
  const [pr, setPr] = useState<CityPackPullRequest | null>(null);
  const propose = useMutation({
    mutationFn: () => api.cityPacks.proposeCityPack({ pack: packFrom(d), note }),
    onSuccess: (r) => setPr(r.pullRequest ?? null),
  });
  const set = (change: Partial<Draft>) => setD({ ...d, ...change });
  const local = d.languages.split(/[\s,]+/).map((l) => l.trim().toLowerCase()).filter((l) => l && l !== 'en');
  const setArea = (i: number, change: Partial<Area>) => set({ areas: d.areas.map((a, j) => (j === i ? { ...a, ...change } : a)) });
  const setOverride = (id: string, change: Partial<Override>) => {
    const was = d.overrides[id] ?? { mode: 'default', name: '', months: [] };
    set({ overrides: { ...d.overrides, [id]: { ...was, ...change } } });
  };
  const byCategory = new Map<string, PackActivity[]>();
  for (const a of activities) byCategory.set(a.category, [...(byCategory.get(a.category) ?? []), a]);

  if (pr) {
    return (
      <section>
        <p><a href="#/cities">← Cities</a></p>
        <div className="card">
          <h1>Pull request opened</h1>
          <p><a href={pr.url} target="_blank" rel="noreferrer">#{pr.number} {pr.title}</a></p>
          <p className="muted">Once it's reviewed and merged, apply the city on the Cities page to load it, then find venues for new or moved areas.</p>
        </div>
      </section>
    );
  }

  return (
    <section className="city-editor">
      <p><a href="#/cities">← Cities</a></p>
      <h1>{isNew ? 'New city' : `Edit ${d.name.en || d.id}`}</h1>

      <div className="card">
        <h2>City</h2>
        <div className="grid">
          <label>Id {isNew ? <span className="muted">(lowercase, e.g. osaka)</span> : null}
            <input value={d.id} disabled={!isNew} onChange={(e) => set({ id: slug(e.target.value) })} />
          </label>
          <label>Name (English)<input value={d.name.en ?? ''} onChange={(e) => set({ name: { ...d.name, en: e.target.value } })} /></label>
          {local.map((l) => (
            <label key={l}>Name ({l}, optional)<input value={d.name[l] ?? ''} onChange={(e) => set({ name: { ...d.name, [l]: e.target.value } })} /></label>
          ))}
          <label>Time zone<input placeholder="Asia/Tokyo" value={d.timezone} onChange={(e) => set({ timezone: e.target.value })} /></label>
          <label>Currency<input placeholder="JPY" maxLength={3} value={d.currency} onChange={(e) => set({ currency: e.target.value.toUpperCase() })} /></label>
          <label>Languages <span className="muted">(ISO 639-1, most common first)</span><input placeholder="ja, en" value={d.languages} onChange={(e) => set({ languages: e.target.value })} /></label>
        </div>
        <div className="row">
          <span>Travel modes:</span>
          <label className="check"><input type="checkbox" checked={d.walk} onChange={(e) => set({ walk: e.target.checked })} /> walking</label>
          <label className="check"><input type="checkbox" checked={d.drive} onChange={(e) => set({ drive: e.target.checked })} /> driving</label>
        </div>
      </div>

      <div className="card">
        <h2>Areas</h2>
        <p className="muted hint">Where people live and meet. Areas already in the repository can be renamed or moved, not removed. New or moved areas need travel times and venues from Google after merging.</p>
        <div className="table-wrap">
          <table>
            <thead><tr><th>Id</th><th>Name (English)</th>{local.map((l) => <th key={l}>Name ({l})</th>)}<th>Latitude</th><th>Longitude</th><th /></tr></thead>
            <tbody>
              {d.areas.map((a, i) => (
                <tr key={i}>
                  <td>{a.existing ? <code>{a.id}</code> : <input value={a.id} onChange={(e) => setArea(i, { id: e.target.value })} />}</td>
                  <td><input value={a.name.en ?? ''} onChange={(e) => {
                    const en = e.target.value;
                    // A new area's id follows its name until edited by hand.
                    const auto = !a.existing && (a.id === '' || a.id === `${d.id}-${slug(a.name.en ?? '')}`);
                    setArea(i, { name: { ...a.name, en }, ...(auto ? { id: `${d.id}-${slug(en)}` } : {}) });
                  }} /></td>
                  {local.map((l) => <td key={l}><input value={a.name[l] ?? ''} onChange={(e) => setArea(i, { name: { ...a.name, [l]: e.target.value } })} /></td>)}
                  <td><input className="coord" value={a.lat} onChange={(e) => setArea(i, { lat: e.target.value })} /></td>
                  <td><input className="coord" value={a.lng} onChange={(e) => setArea(i, { lng: e.target.value })} /></td>
                  <td className="nowrap">
                    <a href={`https://www.google.com/maps/search/?api=1&query=${a.lat},${a.lng}`} target="_blank" rel="noreferrer">Map</a>
                    {!a.existing && <> · <button className="link" onClick={() => set({ areas: d.areas.filter((_, j) => j !== i) })}>Remove</button></>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <button className="secondary small" onClick={() => set({ areas: [...d.areas, { id: '', name: { en: '' }, lat: '', lng: '', existing: false }] })}>Add an area</button>
      </div>

      <div className="card">
        <h2>Activities in {d.name.en || 'this city'}</h2>
        <p className="muted hint">Every activity in the catalogue is offered unless turned off here. Change one to give it a local English name or limit it to its season.</p>
        {[...byCategory].map(([category, acts]) => (
          <div key={category}>
            <h3>{category}</h3>
            <table className="overrides">
              <tbody>
                {acts.map((a) => {
                  const o = d.overrides[a.id] ?? { mode: 'default', name: '', months: [] };
                  return (
                    <tr key={a.id}>
                      <td>{a.name} <span className="muted">{a.id}</span></td>
                      <td>
                        <select value={o.mode} onChange={(e) => setOverride(a.id, { mode: e.target.value as Override['mode'] })}>
                          <option value="default">Offered</option>
                          <option value="custom">Offered, changed</option>
                          <option value="off">Not offered</option>
                        </select>
                      </td>
                      <td>
                        {o.mode === 'custom' && (
                          <div className="row">
                            <input placeholder="Local English name (optional)" value={o.name} onChange={(e) => setOverride(a.id, { name: e.target.value })} />
                            <span className="months">
                              {months.map((m, i) => (
                                <label key={m} className="check">
                                  <input type="checkbox" checked={o.months.includes(i + 1)}
                                    onChange={(e) => setOverride(a.id, { months: e.target.checked ? [...o.months, i + 1].sort((x, y) => x - y) : o.months.filter((x) => x !== i + 1) })} />{m}
                                </label>
                              ))}
                            </span>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ))}
      </div>

      <div className="card">
        <label className="note">Note for the pull request <span className="muted">(optional)</span>
          <textarea rows={2} value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        <div className="row">
          <button disabled={propose.isPending} onClick={() => propose.mutate()}>{propose.isPending ? 'Opening…' : 'Open a pull request'}</button>
          <span className="muted">Checked as CI checks packs. Nothing changes in the app until it's merged and applied.</span>
        </div>
        {propose.error && <pre className="error">{ConnectError.from(propose.error).rawMessage}</pre>}
      </div>
    </section>
  );
}
