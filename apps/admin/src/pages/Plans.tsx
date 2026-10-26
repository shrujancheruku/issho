import { ConnectError, ReviewPlanStatus, type ReviewPlan } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { formatTime, planHref } from '../format';
import { useRememberedCity } from '../city';
import { api } from '../lib';
import { NewPlan } from './NewPlan';

const tabs: { label: string; status: ReviewPlanStatus }[] = [
  { label: 'Drafts', status: ReviewPlanStatus.DRAFT },
  { label: 'Open', status: ReviewPlanStatus.OPEN },
  { label: 'Closed', status: ReviewPlanStatus.CLOSED },
];

/** One city's plans: drafts to review, open and closed ones, and new plans by hand. */
export function Plans() {
  const queryClient = useQueryClient();
  const cities = useQuery({ queryKey: ['review-cities'], queryFn: () => api.review.listReviewCities({}) });
  const [cityId, setCityId] = useRememberedCity('plans.city', cities.data?.cities);
  const [status, setStatus] = useState(ReviewPlanStatus.DRAFT);
  const [creating, setCreating] = useState(false);
  const city = cities.data?.cities.find((c) => c.id === cityId);
  const plans = useQuery({
    queryKey: ['review-plans', cityId, status],
    queryFn: () => api.review.listReviewPlans({ cityId, status }),
    enabled: cityId !== '',
  });
  const toggle = useMutation({
    mutationFn: (review: boolean) => api.review.setCityReview({ cityId, review }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['review-cities'] }),
  });

  return (
    <section>
      <div className="toolbar">
        <h1>Plans</h1>
        <select value={cityId} onChange={(e) => setCityId(e.target.value)}>
          {cities.data?.cities.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        <div className="tabs">
          {tabs.map((t) => (
            <button key={t.label} className={t.status === status ? 'tab active' : 'tab'} onClick={() => setStatus(t.status)}>
              {t.label}{t.status === ReviewPlanStatus.DRAFT && city ? ` (${city.drafts})` : ''}
            </button>
          ))}
        </div>
        <span className="spacer" />
        <button onClick={() => setCreating(!creating)}>{creating ? 'Close the form' : 'New plan'}</button>
      </div>
      {city && (
        <div className="card review-switch">
          <label className="check">
            <input type="checkbox" checked={city.review} disabled={toggle.isPending} onChange={(e) => toggle.mutate(e.target.checked)} />
            <strong>Review plans in {city.name}</strong>
          </label>
          <p className="muted hint">
            {city.review
              ? "On: the matcher's plans wait here as drafts until you approve them. Plans you make by hand open at once."
              : 'Off: the matcher opens plans straight away. Turning it on makes new ones wait here; it doesn’t affect open plans.'}
          </p>
          {toggle.error && <p className="error">{ConnectError.from(toggle.error).message}</p>}
        </div>
      )}
      {creating && city && <NewPlan cityId={city.id} onCreated={() => { setCreating(false); setStatus(ReviewPlanStatus.OPEN); }} />}
      {plans.isPending && cityId && <p className="muted">Loading…</p>}
      {(cities.error || plans.error) && <p className="error">{ConnectError.from(cities.error ?? plans.error).message}</p>}
      {plans.data?.plans.length === 0 && <p className="empty">Nothing here.</p>}
      {plans.data && plans.data.plans.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Plan</th><th>Area</th><th>Languages</th><th>{status === ReviewPlanStatus.DRAFT ? 'Suits' : 'Signed up'}</th><th>Created</th></tr></thead>
            <tbody>{plans.data.plans.map((p) => <PlanRow key={p.id} plan={p} />)}</tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function PlanRow({ plan: p }: { plan: ReviewPlan }) {
  return (
    <tr>
      <td>
        <a href={planHref(p.id)}>{p.title}</a>
        {p.walk && <span className="badge small-badge">walk</span>}
        {p.createdBy && <span className="badge small-badge">by hand</span>}
        {p.closedReason && <span className="muted"> ({p.closedReason})</span>}
      </td>
      <td>{p.areaName}</td>
      <td>{[p.primaryLanguage, p.secondaryLanguage].filter(Boolean).join(' · ')}</td>
      <td>{p.status === ReviewPlanStatus.DRAFT ? `${p.audience} people` : p.signups}</td>
      <td className="nowrap">{formatTime(p.createdAt)}</td>
    </tr>
  );
}
