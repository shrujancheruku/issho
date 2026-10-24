import { ConnectError, ReviewPlanStatus, type ReviewPlan } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { formatTime } from '../format';
import { api } from '../lib';
import { StopsEditor } from './StopsEditor';

const statusLabels: Record<ReviewPlanStatus, string> = {
  [ReviewPlanStatus.UNSPECIFIED]: '',
  [ReviewPlanStatus.DRAFT]: 'Draft',
  [ReviewPlanStatus.OPEN]: 'Open',
  [ReviewPlanStatus.CLOSED]: 'Closed',
};

/** One plan as people would see it (in English), with the review actions. */
export function PlanDetail({ planId }: { planId: string }) {
  const res = useQuery({ queryKey: ['review-plan', planId], queryFn: () => api.review.getReviewPlan({ planId }) });
  const [editing, setEditing] = useState(false);
  if (res.isPending) return <p className="muted">Loading…</p>;
  if (res.error) return <p className="error">{ConnectError.from(res.error).message}</p>;
  const p = res.data.plan!;
  const steps = res.data.steps;

  return (
    <section>
      <p><a href="#/plans">← Plans</a></p>
      <div className="toolbar">
        <h1>{p.title}</h1>
        <span className={`badge status-${statusLabels[p.status].toLowerCase()}`}>{statusLabels[p.status]}{p.closedReason && `: ${p.closedReason}`}</span>
      </div>
      {p.summary && <p>{p.summary}</p>}
      <dl className="ids facts">
        <dt>Where</dt><dd>{p.areaName} ({p.cityId})</dd>
        <dt>Activity</dt><dd>{p.activityId}{p.followUpId && `, then ${p.followUpId}`}{p.walk && ' · composed walk'}</dd>
        <dt>Languages</dt><dd>{[p.primaryLanguage, p.secondaryLanguage].filter(Boolean).join(' · ')}</dd>
        <dt>From</dt><dd>{p.createdBy ? 'made by hand' : `the matcher: suits ${p.audience} people (score ${p.score.toFixed(2)})`}, {formatTime(p.createdAt)}</dd>
        {p.status !== ReviewPlanStatus.DRAFT && <><dt>Round</dt><dd>{p.windowStart}, {p.windowDays} days; signups close {formatTime(p.signupDeadline)}; {p.signups} waiting</dd></>}
        {p.reviewedAt && <><dt>Reviewed</dt><dd>{formatTime(p.reviewedAt)}</dd></>}
      </dl>

      <Actions plan={p} />

      <div className="toolbar">
        <h2>Stops</h2>
        {p.status !== ReviewPlanStatus.CLOSED && !editing && (
          <button className="secondary small" onClick={() => setEditing(true)}>{p.walk ? 'Edit the walk' : 'Change venues'}</button>
        )}
      </div>
      {editing && <StopsEditor data={res.data} onDone={() => setEditing(false)} />}
      {steps.length === 0 ? <p className="muted">No stops: a meeting point is chosen later.</p> : (
        <ol className="stops">
          {steps.map((s, i) => (
            <li key={i}>
              <strong>{s.meeting ? 'Meet' : s.finish ? 'Finish' : s.activityId || 'Stop'}</strong>
              {s.venueName ? <> · {s.venueMapsUrl ? <a href={s.venueMapsUrl} target="_blank" rel="noreferrer">{s.venueName}</a> : s.venueName}</> : <span className="muted"> · no venue</span>}
              {s.venueAddress && <span className="muted"> · {s.venueAddress}</span>}
              {(s.minutes > 0 || s.walkMinutes > 0) && <span className="muted"> · {s.walkMinutes > 0 && `${s.walkMinutes} min walk there`}{s.minutes > 0 && s.walkMinutes > 0 && ', '}{s.minutes > 0 && `${s.minutes} min here`}</span>}
              {s.note && <p className="meta">{s.note}</p>}
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

function Actions({ plan: p }: { plan: ReviewPlan }) {
  const queryClient = useQueryClient();
  const [title, setTitle] = useState(p.storedTitle);
  const [summary, setSummary] = useState(p.summary);
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['review-plan', p.id] });
    void queryClient.invalidateQueries({ queryKey: ['review-plans'] });
    void queryClient.invalidateQueries({ queryKey: ['review-cities'] });
  };
  const act = useMutation({
    mutationFn: async (what: 'save' | 'approve' | 'reject' | 'withdraw') => {
      if (what === 'save' || (what === 'approve' && (title !== p.storedTitle || summary !== p.summary))) {
        await api.review.editPlan({ planId: p.id, title, summary });
      }
      if (what === 'approve') await api.review.approvePlan({ planId: p.id });
      if (what === 'reject') await api.review.rejectPlan({ planId: p.id });
      if (what === 'withdraw') await api.review.withdrawPlan({ planId: p.id });
    },
    onSuccess: refresh,
  });
  if (p.status === ReviewPlanStatus.CLOSED) return null;
  const changed = title !== p.storedTitle || summary !== p.summary;

  return (
    <article className="card decision">
      <label className="note">
        Title <span className="muted">(English; empty = made from the activities)</span>
        <input maxLength={80} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={p.storedTitle ? '' : p.title} />
      </label>
      <label className="note">
        Summary <span className="muted">(English; shown on the plan, translated for each reader)</span>
        <textarea rows={2} maxLength={500} value={summary} onChange={(e) => setSummary(e.target.value)} />
      </label>
      <div className="row">
        {p.status === ReviewPlanStatus.DRAFT ? (
          <>
            <button disabled={act.isPending} onClick={() => act.mutate('approve')}>{changed ? 'Save and approve' : 'Approve'}</button>
            <button className="secondary" disabled={act.isPending} onClick={() => act.mutate('reject')}>Reject</button>
            {changed && <button className="secondary" disabled={act.isPending} onClick={() => act.mutate('save')}>Save</button>}
          </>
        ) : (
          <>
            <button disabled={act.isPending || !changed} onClick={() => act.mutate('save')}>Save</button>
            <button className="secondary" disabled={act.isPending} onClick={() => act.mutate('withdraw')}>Withdraw plan</button>
          </>
        )}
      </div>
      <p className="muted hint">
        {p.status === ReviewPlanStatus.DRAFT
          ? 'Approving opens it with signups from today. Rejecting stops the matcher suggesting it again for two weeks.'
          : 'Withdrawing closes it: people who signed up see it closed (no notification); groups already scheduled go ahead.'}
      </p>
      {act.error && <p className="error">{ConnectError.from(act.error).message}</p>}
    </article>
  );
}
