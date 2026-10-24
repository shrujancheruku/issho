import { ConnectError, type FoundPlace, type GetReviewPlanResponse, type ReviewPlan } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';

import { api } from '../lib';

type Row = { placeId: string; name: string; minutes: number; note: string };

const roleOf = (i: number, n: number) => (i === 0 ? 'Meet' : i === n - 1 ? 'Finish' : `Stop ${i}`);

/** Edits a walk's stops: order, places, time at each and notes; or another plan's venues. */
export function StopsEditor({ data, onDone }: { data: GetReviewPlanResponse; onDone: () => void }) {
  const p = data.plan!;
  return p.walk ? <WalkEditor data={data} onDone={onDone} /> : <VenueEditor data={data} onDone={onDone} />;
}

function useRefresh(planId: string) {
  const queryClient = useQueryClient();
  return () => {
    void queryClient.invalidateQueries({ queryKey: ['review-plan', planId] });
    void queryClient.invalidateQueries({ queryKey: ['review-plans'] });
  };
}

function Upcoming({ plan, count }: { plan: ReviewPlan; count: number }) {
  if (!count) return null;
  return (
    <p className="error">
      {count === 1 ? 'An outing of this plan is' : `${count} outings of this plan are`} already scheduled: the people going see the change.
      {' '}Opening hours were checked for the old {plan.walk ? 'stops' : 'venues'} only.
    </p>
  );
}

function WalkEditor({ data, onDone }: { data: GetReviewPlanResponse; onDone: () => void }) {
  const p = data.plan!;
  const refresh = useRefresh(p.id);
  // Steps and stops are in the same order; names come from the steps.
  const [rows, setRows] = useState<Row[]>(() =>
    data.stops.map((s, i) => ({ placeId: s.placeId, name: data.steps[i]?.venueName || s.placeId, minutes: s.minutes, note: s.note })));
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<{ warnings: string[]; minutes: number } | null>(null);
  const find = useMutation({ mutationFn: (q: string) => api.review.findPlanPlaces({ planId: p.id, query: q }) });
  const save = useMutation({
    mutationFn: () => api.review.setPlanStops({
      planId: p.id, stops: rows.map((r) => ({ placeId: r.placeId, minutes: r.minutes, note: r.note })),
    }),
    onSuccess: (res) => {
      setResult({ warnings: res.warnings, minutes: res.minutes });
      refresh();
    },
  });
  const update = (i: number, change: Partial<Row>) => setRows(rows.map((r, j) => (j === i ? { ...r, ...change } : r)));
  const move = (i: number, by: number) => {
    const next = [...rows];
    [next[i], next[i + by]] = [next[i + by], next[i]];
    setRows(next);
  };
  // A new place goes before the finish; move it from there.
  const add = (f: FoundPlace) => setRows([...rows.slice(0, -1), { placeId: f.placeId, name: f.name, minutes: 20, note: '' }, ...rows.slice(-1)]);
  const search = (e: FormEvent) => {
    e.preventDefault();
    if (query.trim()) find.mutate(query.trim());
  };
  const error = save.error ?? find.error;

  return (
    <article className="card stops-editor">
      <h2>Edit the walk</h2>
      <Upcoming plan={p} count={data.upcomingEvents} />
      <table>
        <thead><tr><th /><th>Place</th><th>Minutes there</th><th>Note <span className="muted">(English)</span></th><th /></tr></thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={r.placeId}>
              <td className="nowrap"><strong>{roleOf(i, rows.length)}</strong></td>
              <td>{r.name}</td>
              <td><input type="number" min={0} max={120} value={r.minutes} onChange={(e) => update(i, { minutes: Number(e.target.value) })} /></td>
              <td><textarea rows={2} maxLength={300} value={r.note} onChange={(e) => update(i, { note: e.target.value })} /></td>
              <td className="nowrap">
                <button type="button" className="small secondary" aria-label="Move up" disabled={i === 0} onClick={() => move(i, -1)}>↑</button>{' '}
                <button type="button" className="small secondary" aria-label="Move down" disabled={i === rows.length - 1} onClick={() => move(i, 1)}>↓</button>{' '}
                <button type="button" className="small secondary" disabled={rows.length <= 3} onClick={() => setRows(rows.filter((_, j) => j !== i))}>Remove</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <form className="row" onSubmit={search}>
        <input type="search" placeholder="Find a place to add, near the area" value={query} onChange={(e) => setQuery(e.target.value)} />
        <button className="secondary" disabled={find.isPending || !query.trim()}>Find</button>
      </form>
      {find.data && (
        find.data.places.length === 0 ? <p className="empty">Google found nothing within 3 km.</p> : (
          <ul className="found">
            {find.data.places.map((f) => {
              const taken = rows.some((r) => r.placeId === f.placeId);
              return (
                <li key={f.placeId}>
                  <a href={f.mapsUrl} target="_blank" rel="noreferrer">{f.name}</a>
                  <span className="muted"> · {f.address} · {f.primaryType || 'place'} · {(f.meters / 1000).toFixed(1)} km from the centre</span>
                  {f.closed ? <span className="badge small-badge">closed</span>
                    : f.meters > 3000 ? <span className="muted"> · too far</span>
                    : <button type="button" className="small" disabled={taken} onClick={() => add(f)}>{taken ? 'In the walk' : 'Add'}</button>}
                </li>
              );
            })}
          </ul>
        )
      )}
      <div className="row">
        <button disabled={save.isPending} onClick={() => save.mutate()}>Save the walk</button>
        <button className="secondary" onClick={onDone}>Done</button>
        <span className="muted">The first place is where people meet, the last where they sit down at the end. Saving works out the walking route again in this order.</span>
      </div>
      {result && (
        <div className={result.warnings.length ? 'warnings' : undefined}>
          <p>Saved{result.minutes ? `: about ${Math.floor(result.minutes / 60)} h ${result.minutes % 60} min in all` : ''}.</p>
          {result.warnings.length > 0 && <ul>{result.warnings.map((w) => <li key={w}>{w}</li>)}</ul>}
        </div>
      )}
      {error && <p className="error">{ConnectError.from(error).message}</p>}
    </article>
  );
}

function VenueEditor({ data, onDone }: { data: GetReviewPlanResponse; onDone: () => void }) {
  const p = data.plan!;
  return (
    <article className="card stops-editor">
      <h2>Change the venues</h2>
      <Upcoming plan={p} count={data.upcomingEvents} />
      <VenuePicker plan={p} activityId={p.activityId} current={data.venuePlaceId} followUp={false} />
      {p.followUpId && <VenuePicker plan={p} activityId={p.followUpId} current={data.followUpVenuePlaceId} followUp />}
      <div className="row">
        <button className="secondary" onClick={onDone}>Done</button>
        <span className="muted">Venues come from those found for the activity in the area (approve more on the Venues page).</span>
      </div>
    </article>
  );
}

function VenuePicker({ plan, activityId, current, followUp }: { plan: ReviewPlan; activityId: string; current: string; followUp: boolean }) {
  const refresh = useRefresh(plan.id);
  const [placeId, setPlaceId] = useState(current);
  const options = useQuery({
    queryKey: ['venue-options', plan.areaId, activityId],
    queryFn: () => api.review.listVenueOptions({ areaId: plan.areaId, activityId }),
  });
  const save = useMutation({
    mutationFn: async () => {
      await api.review.setPlanVenue({ planId: plan.id, placeId, followUp });
    },
    onSuccess: refresh,
  });
  const venues = options.data?.venues ?? [];
  return (
    <div className="row">
      <label>{followUp ? 'Then' : 'Venue'} ({activityId})
        <select value={placeId} onChange={(e) => setPlaceId(e.target.value)} disabled={!venues.length}>
          <option value="">{options.data && !venues.length ? 'None found: a meeting point instead' : 'Pick one for me'}</option>
          {venues.map((v) => <option key={v.placeId} value={v.placeId}>{v.name || v.placeId}{v.status === 'approved' ? ' ✓' : ''}</option>)}
          {/* The current venue may no longer be among the options (rejected since). */}
          {current && !venues.some((v) => v.placeId === current) && <option value={current}>Current venue</option>}
        </select>
      </label>
      <button disabled={save.isPending || placeId === current} onClick={() => save.mutate()}>Save</button>
      {save.isSuccess && placeId === current && <span className="muted">Saved.</span>}
      {(save.error || options.error) && <span className="error">{ConnectError.from(save.error ?? options.error).message}</span>}
    </div>
  );
}
