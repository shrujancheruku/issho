import { ConnectError, type WeekMetrics } from '@issho/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { api } from '../lib';

const weekOptions = [8, 12, 26, 52];

const pct = (n: number, of: number) => (of ? `${Math.round((100 * n) / of)}%` : '–');
const rating = (r: number) => (r ? r.toFixed(1) : '–');
const shortWeek = (w: string) => new Date(`${w}T00:00:00Z`).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });

/** Per-city domain metrics, led by the north star: weekly attendees who've been to more than one outing. */
export function Metrics() {
  const cities = useQuery({ queryKey: ['review-cities'], queryFn: () => api.review.listReviewCities({}) });
  const [cityId, setCityId] = useState(() => localStorage.getItem('metrics.city') ?? '');
  const [weeks, setWeeks] = useState(12);
  useEffect(() => {
    const list = cities.data?.cities ?? [];
    if (list.length && !list.some((c) => c.id === cityId)) setCityId(list[0].id);
  }, [cities.data, cityId]);
  useEffect(() => {
    if (cityId) localStorage.setItem('metrics.city', cityId);
  }, [cityId]);
  const metrics = useQuery({
    queryKey: ['metrics', cityId, weeks],
    queryFn: () => api.metrics.getCityMetrics({ cityId, weeks }),
    enabled: cityId !== '',
  });
  const rows = metrics.data?.weeks ?? [];
  const now = metrics.data?.now;
  // The last row is the week in progress; the one before is the last full week.
  const current = rows.at(-1);
  const lastFull = rows.at(-2);

  return (
    <section>
      <div className="toolbar">
        <h1>Metrics</h1>
        <select value={cityId} onChange={(e) => setCityId(e.target.value)}>
          {cities.data?.cities.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <select value={weeks} onChange={(e) => setWeeks(Number(e.target.value))}>
          {weekOptions.map((w) => <option key={w} value={w}>Last {w} weeks</option>)}
        </select>
      </div>
      {metrics.isPending && cityId && <p className="muted">Loading…</p>}
      {(cities.error || metrics.error) && <p className="error">{ConnectError.from(cities.error ?? metrics.error).message}</p>}
      {metrics.data && (
        <>
          <div className="tiles">
            <div className="tile hero">
              <span className="muted">Came back last full week</span>
              <strong>{lastFull?.repeatAttendees ?? 0}</strong>
              <span className="muted">of {lastFull?.attendees ?? 0} who went, week of {lastFull ? shortWeek(lastFull.week) : '–'}</span>
            </div>
            <div className="tile">
              <span className="muted">This week so far</span>
              <strong>{current?.repeatAttendees ?? 0}</strong>
              <span className="muted">of {current?.attendees ?? 0} who went</span>
            </div>
            {now && (
              <>
                <div className="tile"><span className="muted">People in the city</span><strong>{now.people}</strong></div>
                <div className="tile"><span className="muted">Open plans</span><strong>{now.openPlans}</strong><span className="muted">{now.waiting} waiting</span></div>
                <div className="tile"><span className="muted">Outings to come</span><strong>{now.upcomingOutings}</strong></div>
                <div className="tile"><span className="muted">Active crews</span><strong>{now.activeCrews}</strong></div>
              </>
            )}
          </div>
          <div className="card">
            <h2>Weekly attendees</h2>
            <p className="muted hint">People who went to an outing each week. The north star is those who came back: two or more outings so far. The last week is still in progress.</p>
            <AttendeeChart rows={rows} />
          </div>
          <WeekTable rows={rows} />
        </>
      )}
    </section>
  );
}

const W = 720;
const H = 240;
const pad = { top: 16, right: 8, bottom: 28, left: 32 };

/** Stacked bars: came back (the north star) under one outing so far, per week. */
function AttendeeChart({ rows }: { rows: WeekMetrics[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const max = Math.max(4, ...rows.map((r) => r.attendees));
  // A round axis top: 4 ticks.
  const step = Math.ceil(max / 4);
  const top = step * 4;
  const plotW = W - pad.left - pad.right;
  const plotH = H - pad.top - pad.bottom;
  const slot = plotW / Math.max(rows.length, 1);
  const barW = Math.min(36, slot * 0.6);
  const y = (v: number) => pad.top + plotH - (v / top) * plotH;
  const gap = 2;
  const h = hover === null ? undefined : rows[hover];

  return (
    <div className="chart">
      <div className="legend">
        <span><i className="swatch back" /> Came back (2+ outings)</span>
        <span><i className="swatch once" /> One outing so far</span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label="Weekly attendees: came back and one outing so far" onMouseLeave={() => setHover(null)}>
        {[0, 1, 2, 3, 4].map((i) => (
          <g key={i}>
            <line className="grid" x1={pad.left} x2={W - pad.right} y1={y(i * step)} y2={y(i * step)} />
            <text className="tick" x={pad.left - 6} y={y(i * step) + 4} textAnchor="end">{i * step}</text>
          </g>
        ))}
        {rows.map((r, i) => {
          const x = pad.left + i * slot + (slot - barW) / 2;
          const back = r.repeatAttendees;
          const once = r.attendees - r.repeatAttendees;
          const yBack = y(back);
          const yAll = y(r.attendees);
          const showLabel = i % Math.ceil(rows.length / 8) === 0 || i === rows.length - 1;
          return (
            <g key={r.week} onMouseEnter={() => setHover(i)}>
              {/* Hit target: the whole column. */}
              <rect x={pad.left + i * slot} y={pad.top} width={slot} height={plotH} fill="transparent" />
              {back > 0 && <path className="bar back" d={barPath(x, yBack, barW, y(0) - yBack, once === 0)} />}
              {once > 0 && <path className="bar once" d={barPath(x, yAll, barW, yBack - yAll - (back > 0 ? gap : 0), true)} />}
              {hover === i && <rect className="hover-col" x={pad.left + i * slot} y={pad.top} width={slot} height={plotH} />}
              {showLabel && <text className="tick" x={x + barW / 2} y={H - 8} textAnchor="middle">{shortWeek(r.week)}</text>}
            </g>
          );
        })}
        <line className="axis" x1={pad.left} x2={W - pad.right} y1={y(0)} y2={y(0)} />
      </svg>
      {h && (
        <div className="tooltip" style={{ left: `${((pad.left + (hover! + 0.5) * slot) / W) * 100}%` }}>
          <strong>Week of {shortWeek(h.week)}{hover === rows.length - 1 ? ' (so far)' : ''}</strong>
          <span><i className="swatch back" /> Came back: {h.repeatAttendees}</span>
          <span><i className="swatch once" /> One outing so far: {h.attendees - h.repeatAttendees}</span>
          <span className="muted">{h.attendees} went · {h.firstTimers} first outing</span>
        </div>
      )}
    </div>
  );
}

/** A bar from its top-left, with 4px rounded top corners when it's the top of the stack. */
function barPath(x: number, top: number, w: number, h: number, rounded: boolean) {
  if (h <= 0) return '';
  const r = rounded ? Math.min(4, w / 2, h) : 0;
  return `M${x},${top + h} V${top + r} Q${x},${top} ${x + r},${top} H${x + w - r} Q${x + w},${top} ${x + w},${top + r} V${top + h} Z`;
}

function WeekTable({ rows }: { rows: WeekMetrics[] }) {
  return (
    <div className="table-wrap">
      <table className="numbers">
        <thead>
          <tr>
            <th>Week of</th><th>Went</th><th>Came back</th><th>First outing</th><th>Groups</th>
            <th title="Said they'd go, of those invited">Said yes</th><th title="Went, of those who said they'd go">Showed up</th>
            <th>Plan / group rating</th><th title="Meet-again picks, of all picks">Meet again</th><th>Mutual pairs</th>
            <th>New people</th><th>Crews formed</th><th>Reports</th>
          </tr>
        </thead>
        <tbody>
          {[...rows].reverse().map((r, i) => (
            <tr key={r.week}>
              <td className="nowrap">{shortWeek(r.week)}{i === 0 ? ' (so far)' : ''}</td>
              <td>{r.attendees}</td>
              <td><strong>{r.repeatAttendees}</strong></td>
              <td>{r.firstTimers}</td>
              <td>{r.groups}</td>
              <td>{pct(r.confirmed, r.invited)}</td>
              <td>{pct(r.attended, r.confirmed)}</td>
              <td>{rating(r.planRating)} / {rating(r.groupRating)}</td>
              <td>{pct(r.picksAgain, r.picks)}</td>
              <td>{r.mutualPairs}</td>
              <td>{r.newPeople}</td>
              <td>{r.crewsFormed}</td>
              <td>{r.reports}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
