import { ConnectError, VenueStatus, type FoundVenue, type Venue, type VenueCity } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { formatTime } from '../format';
import { useRememberedCity } from '../city';
import { api } from '../lib';

const PAGE = 50;

const tabs: { label: string; status: VenueStatus; count: (c: VenueCity) => number }[] = [
  { label: 'To review', status: VenueStatus.CANDIDATE, count: (c) => c.candidates },
  { label: 'Approved', status: VenueStatus.APPROVED, count: (c) => c.approved },
  { label: 'Rejected', status: VenueStatus.REJECTED, count: (c) => c.rejected },
];

const statusLabel: Record<VenueStatus, string> = {
  [VenueStatus.UNSPECIFIED]: '',
  [VenueStatus.CANDIDATE]: 'to review',
  [VenueStatus.APPROVED]: 'approved',
  [VenueStatus.REJECTED]: 'rejected',
};

/** One city's venues: approve or reject what discovery found, find places by name, add missing ones. */
export function Venues() {
  const queryClient = useQueryClient();
  const cities = useQuery({ queryKey: ['venue-cities'], queryFn: () => api.curation.listVenueCities({}) });
  const [cityId, setCityId] = useRememberedCity('venues.city', cities.data?.cities);
  const [status, setStatus] = useState(VenueStatus.CANDIDATE);
  const [areaId, setAreaId] = useState('');
  const [categoryId, setCategoryId] = useState('');
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  // A new filter starts from the first page with nothing selected.
  const refilter = <T,>(set: (value: T) => void) => (value: T) => {
    set(value);
    setOffset(0);
    setSelected(new Set());
  };
  const city = cities.data?.cities.find((c) => c.id === cityId);
  const categories = cities.data?.categories ?? [];
  const categoryName = (id: string) => categories.find((c) => c.id === id)?.name ?? id;
  const venues = useQuery({
    queryKey: ['venues', cityId, status, areaId, categoryId, offset],
    queryFn: () => api.curation.listVenues({ cityId, status, areaId, categoryId, offset }),
    enabled: cityId !== '',
  });
  const refresh = () => {
    setSelected(new Set());
    void queryClient.invalidateQueries({ queryKey: ['venues'] });
    void queryClient.invalidateQueries({ queryKey: ['venue-cities'] });
  };
  const toggle = useMutation({
    mutationFn: (on: boolean) => api.curation.setApprovedVenuesOnly({ cityId, approvedVenuesOnly: on }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['venue-cities'] }),
  });
  const setStatusOf = useMutation({
    mutationFn: async ({ ids, to }: { ids: string[]; to: VenueStatus }) => {
      await api.curation.setVenueStatus({ cityId, placeIds: ids, status: to });
    },
    onSuccess: refresh,
  });

  const rows = venues.data?.venues ?? [];
  const total = venues.data?.total ?? 0;
  const allChecked = rows.length > 0 && rows.every((v) => selected.has(v.placeId));
  const check = (id: string, on: boolean) => {
    const next = new Set(selected);
    if (on) next.add(id); else next.delete(id);
    setSelected(next);
  };
  const others = tabs.filter((t) => t.status !== status);

  return (
    <section>
      <div className="toolbar">
        <h1>Venues</h1>
        <select value={cityId} onChange={(e) => { refilter(setCityId)(e.target.value); setAreaId(''); }}>
          {cities.data?.cities.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <div className="tabs">
          {tabs.map((t) => (
            <button key={t.label} className={t.status === status ? 'tab active' : 'tab'} onClick={() => refilter(setStatus)(t.status)}>
              {t.label}{city ? ` (${t.count(city)})` : ''}
            </button>
          ))}
        </div>
        <select value={areaId} onChange={(e) => refilter(setAreaId)(e.target.value)}>
          <option value="">All areas</option>
          {city?.areas.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
        <select value={categoryId} onChange={(e) => refilter(setCategoryId)(e.target.value)}>
          <option value="">All kinds</option>
          {categories.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </div>
      {city && (
        <div className="card review-switch">
          <label className="check">
            <input type="checkbox" checked={city.approvedVenuesOnly} disabled={toggle.isPending} onChange={(e) => toggle.mutate(e.target.checked)} />
            <strong>Use only approved venues in {city.name}</strong>
          </label>
          <p className="muted hint">
            {city.approvedVenuesOnly
              ? `On: new plans use only the ${city.approved} approved places, and plans with no approved place of the right kind aren't suggested. Plans already made keep their venues.`
              : `Off: new plans use approved places first, then any place not rejected (${city.candidates} still to review). Turn it on once enough are approved.`}
          </p>
          {toggle.error && <p className="error">{ConnectError.from(toggle.error).message}</p>}
        </div>
      )}
      {city && <Search city={city} categoryName={categoryName} onChanged={refresh} />}
      {venues.isPending && cityId && <p className="muted">Loading…</p>}
      {(cities.error || venues.error || setStatusOf.error) && (
        <p className="error">{ConnectError.from(cities.error ?? venues.error ?? setStatusOf.error).message}</p>
      )}
      {venues.data && rows.length === 0 && <p className="empty">Nothing here.</p>}
      {rows.length > 0 && (
        <>
          <div className="row">
            <span className="muted">{selected.size} selected</span>
            {others.map((t) => (
              <button key={t.status} className="small secondary" disabled={!selected.size || setStatusOf.isPending}
                onClick={() => setStatusOf.mutate({ ids: [...selected], to: t.status })}>
                {t.status === VenueStatus.CANDIDATE ? 'Back to review' : t.status === VenueStatus.APPROVED ? 'Approve' : 'Reject'} selected
              </button>
            ))}
            <span className="spacer" />
            <span className="muted">Names are fetched from Google each time; a <span className="badge small-badge flag">flag</span> means look closer.</span>
          </div>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th><input type="checkbox" aria-label="Select all" checked={allChecked} onChange={(e) => setSelected(e.target.checked ? new Set(rows.map((v) => v.placeId)) : new Set())} /></th>
                  <th>Place</th><th>Area</th><th>Kind</th><th>In plans</th><th>{status === VenueStatus.CANDIDATE ? '' : 'Reviewed'}</th><th />
                </tr>
              </thead>
              <tbody>
                {rows.map((v) => (
                  <VenueRow key={v.placeId} venue={v} checked={selected.has(v.placeId)} onCheck={(on) => check(v.placeId, on)}
                    categoryName={categoryName} busy={setStatusOf.isPending}
                    onSet={(to) => setStatusOf.mutate({ ids: [v.placeId], to })} />
                ))}
              </tbody>
            </table>
          </div>
          <div className="row pager">
            <button className="secondary small" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE))}>Previous</button>
            <span className="muted">{offset + 1}–{offset + rows.length} of {total}</span>
            <button className="secondary small" disabled={offset + PAGE >= total} onClick={() => setOffset(offset + PAGE)}>Next</button>
          </div>
        </>
      )}
    </section>
  );
}

function PlaceName({ name, placeId, mapsUrl, address }: { name: string; placeId: string; mapsUrl: string; address: string }) {
  return (
    <>
      {mapsUrl ? <a href={mapsUrl} target="_blank" rel="noreferrer">{name || placeId}</a> : <span>{name || placeId}</span>}
      {address && <div className="muted address">{address}</div>}
    </>
  );
}

function VenueRow({ venue: v, checked, onCheck, categoryName, busy, onSet }: {
  venue: Venue; checked: boolean; onCheck: (on: boolean) => void; categoryName: (id: string) => string;
  busy: boolean; onSet: (to: VenueStatus) => void;
}) {
  return (
    <tr className={v.flag ? 'flagged' : undefined}>
      <td><input type="checkbox" aria-label={`Select ${v.name || v.placeId}`} checked={checked} onChange={(e) => onCheck(e.target.checked)} /></td>
      <td>
        <PlaceName name={v.name} placeId={v.placeId} mapsUrl={v.mapsUrl} address={v.address} />
        {v.flag && <span className="badge small-badge flag">{v.flag}</span>}
        {v.added && <span className="badge small-badge">added</span>}
      </td>
      <td>{v.areaName}</td>
      <td>{v.categoryIds.map(categoryName).join(', ')}</td>
      <td>{v.plans || ''}</td>
      <td className="nowrap muted">{v.reviewedAt ? formatTime(v.reviewedAt) : ''}</td>
      <td className="nowrap">
        {v.status !== VenueStatus.APPROVED && <button className="small" disabled={busy} onClick={() => onSet(VenueStatus.APPROVED)}>Approve</button>}{' '}
        {v.status !== VenueStatus.REJECTED && <button className="small secondary" disabled={busy} onClick={() => onSet(VenueStatus.REJECTED)}>Reject</button>}
      </td>
    </tr>
  );
}

/** Look places up by name with Google (e.g. スナック to find snack bars), act on known ones, add missing ones. */
function Search({ city, categoryName, onChanged }: { city: VenueCity; categoryName: (id: string) => string; onChanged: () => void }) {
  const [query, setQuery] = useState('');
  const [areaId, setAreaId] = useState('');
  const [category, setCategory] = useState<Record<string, string>>({});
  // Google finds far more snack bars than discovery kept (it ranks by
  // popularity), so the sweep starts with only the places we have.
  const [onlyOurs, setOnlyOurs] = useState(false);
  const search = useMutation({
    mutationFn: (q: string) => api.curation.searchVenues({ cityId: city.id, areaId, query: q }),
  });
  // A search's results stay on screen; acting on one runs it again so
  // statuses are current.
  const rerun = () => {
    onChanged();
    if (search.variables) search.mutate(search.variables);
  };
  const setStatusOf = useMutation({
    mutationFn: async ({ id, to }: { id: string; to: VenueStatus }) => {
      await api.curation.setVenueStatus({ cityId: city.id, placeIds: [id], status: to });
    },
    onSuccess: rerun,
  });
  const add = useMutation({
    mutationFn: async (v: FoundVenue) => {
      await api.curation.addVenue({ cityId: city.id, placeId: v.placeId, categoryId: category[v.placeId] ?? v.addableCategoryIds[0] });
    },
    onSuccess: rerun,
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    setOnlyOurs(false);
    search.mutate(query.trim());
  };
  const searches = areaId ? 1 : city.areas.length;
  const error = search.error ?? setStatusOf.error ?? add.error;
  const shown = (search.data?.venues ?? []).filter((v) => v.known || !onlyOurs);

  return (
    <form className="card" onSubmit={submit}>
      <div className="row">
        <input type="search" placeholder="Find places by name, e.g. スナック" value={query} onChange={(e) => setQuery(e.target.value)} />
        <select value={areaId} onChange={(e) => setAreaId(e.target.value)}>
          <option value="">Every area</option>
          {city.areas.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
        <button disabled={search.isPending || !query.trim()}>Search</button>
        <button type="button" className="secondary" disabled={search.isPending} onClick={() => { setQuery('スナック'); setOnlyOurs(true); search.mutate('スナック'); }}>
          Look for snack bars
        </button>
        <span className="muted">{searches} Google {searches === 1 ? 'search' : 'searches'}</span>
      </div>
      {search.isPending && <p className="muted">Searching…</p>}
      {error && <p className="error">{ConnectError.from(error).message}</p>}
      {search.data && search.data.venues.length > 0 && (
        <label className="check">
          <input type="checkbox" checked={onlyOurs} onChange={(e) => setOnlyOurs(e.target.checked)} />
          Only places we have ({search.data.venues.filter((v) => v.known).length} of {search.data.venues.length} found)
        </label>
      )}
      {search.data && shown.length === 0 && <p className="empty">{onlyOurs ? 'None of these are ours.' : 'Google found nothing.'}</p>}
      {shown.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Place</th><th>Area</th><th>Here</th><th /></tr></thead>
            <tbody>
              {shown.map((v) => (
                <tr key={v.placeId} className={v.flag ? 'flagged' : undefined}>
                  <td>
                    <PlaceName name={v.name} placeId={v.placeId} mapsUrl={v.mapsUrl} address={v.address} />
                    {v.flag && <span className="badge small-badge flag">{v.flag}</span>}
                    {v.closed && <span className="badge small-badge">closed</span>}
                  </td>
                  <td>{v.areaName}</td>
                  <td>{v.known ? `${statusLabel[v.status]} · ${v.categoryIds.map(categoryName).join(', ')}` : <span className="muted">not one of ours</span>}</td>
                  <td className="nowrap">
                    {v.known && v.status !== VenueStatus.APPROVED && (
                      <button type="button" className="small" disabled={setStatusOf.isPending} onClick={() => setStatusOf.mutate({ id: v.placeId, to: VenueStatus.APPROVED })}>Approve</button>
                    )}{' '}
                    {v.known && v.status !== VenueStatus.REJECTED && (
                      <button type="button" className="small secondary" disabled={setStatusOf.isPending} onClick={() => setStatusOf.mutate({ id: v.placeId, to: VenueStatus.REJECTED })}>Reject</button>
                    )}
                    {!v.known && v.addableCategoryIds.length > 0 && (
                      <>
                        <select value={category[v.placeId] ?? v.addableCategoryIds[0]} onChange={(e) => setCategory({ ...category, [v.placeId]: e.target.value })}>
                          {v.addableCategoryIds.map((c) => <option key={c} value={c}>{categoryName(c)}</option>)}
                        </select>{' '}
                        <button type="button" className="small" disabled={add.isPending} onClick={() => add.mutate(v)}>Add, approved</button>
                      </>
                    )}
                    {!v.known && !v.closed && v.addableCategoryIds.length === 0 && <span className="muted">fits no kind of venue</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </form>
  );
}
