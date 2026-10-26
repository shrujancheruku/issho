import { ConnectError, ReviewEventStatus, type ReviewEvent } from '@issho/api-client';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { eventHref, localTime } from '../format';
import { useRememberedCity } from '../city';
import { api } from '../lib';

const tabs: { label: string; status: ReviewEventStatus }[] = [
  { label: 'Proposed', status: ReviewEventStatus.PROPOSED },
  { label: 'Upcoming', status: ReviewEventStatus.SCHEDULED },
];

/** A city's proposed rounds to review, and upcoming events (for meeting points). */
export function Groups() {
  const cities = useQuery({ queryKey: ['review-cities'], queryFn: () => api.review.listReviewCities({}) });
  const [cityId, setCityId] = useRememberedCity('plans.city', cities.data?.cities);
  const [status, setStatus] = useState(ReviewEventStatus.PROPOSED);
  const city = cities.data?.cities.find((c) => c.id === cityId);
  const events = useQuery({
    queryKey: ['review-events', cityId, status],
    queryFn: () => api.review.listReviewEvents({ cityId, status }),
    enabled: cityId !== '',
  });

  return (
    <section>
      <div className="toolbar">
        <h1>Groups</h1>
        <select value={cityId} onChange={(e) => setCityId(e.target.value)}>
          {cities.data?.cities.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <div className="tabs">
          {tabs.map((t) => (
            <button key={t.label} className={t.status === status ? 'tab active' : 'tab'} onClick={() => setStatus(t.status)}>{t.label}</button>
          ))}
        </div>
      </div>
      <p className="muted hint">
        {status === ReviewEventStatus.PROPOSED
          ? city?.review
            ? `When a plan in ${city.name} is ready to schedule, its date and groups wait here. Nobody is invited until you approve.`
            : `${city?.name ?? 'This city'} isn't under review (Plans page), so rounds are scheduled straight away.`
          : 'Scheduled outings from today. Set where each group meets; people who confirmed see it 24 hours before.'}
      </p>
      {(cities.error || events.error) && <p className="error">{ConnectError.from(cities.error ?? events.error).message}</p>}
      {events.data?.events.length === 0 && <p className="empty">Nothing here.</p>}
      {events.data && events.data.events.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>When</th><th>Plan</th><th>Area</th><th>Groups</th><th>People</th></tr></thead>
            <tbody>{events.data.events.map((e) => <EventRow key={e.id} event={e} />)}</tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function EventRow({ event: e }: { event: ReviewEvent }) {
  return (
    <tr>
      <td className="nowrap"><a href={eventHref(e.id)}>{localTime(e.startsAt, e.timeZone) || `${e.date} ${e.block}`}</a></td>
      <td>{e.title}{e.forced && <span className="badge small-badge">at deadline</span>}</td>
      <td>{e.areaName}</td>
      <td>{e.groups}</td>
      <td>{e.people}</td>
    </tr>
  );
}
