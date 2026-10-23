import { ConnectError, FlagKind, OutingFeedback, type Person as PersonData } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { formatTime, leanSummary } from '../format';
import { api } from '../lib';

const feedbackLabels: Record<OutingFeedback, string> = {
  [OutingFeedback.UNSPECIFIED]: '',
  [OutingFeedback.SAID_WENT]: 'said they went',
  [OutingFeedback.SAID_DIDNT_GO]: "said they didn't go",
};

/** Everything a moderator needs to decide about one person, and the decision. */
export function Person({ userId }: { userId: string }) {
  const person = useQuery({ queryKey: ['person', userId], queryFn: () => api.moderation.getPerson({ userId }) });

  if (person.isPending) return <p className="muted">Loading…</p>;
  if (person.error) return <p className="error">{ConnectError.from(person.error).message}</p>;
  const p = person.data.person!;
  const age = p.birthYear ? new Date().getFullYear() - p.birthYear : null;

  return (
    <section>
      <p><a href="#/people">← People</a></p>
      <div className="toolbar">
        <h1>{p.name || 'No profile'}</h1>
        {p.suspended && <span className="badge suspended">Suspended</span>}
      </div>
      <dl className="ids facts">
        <dt>Email</dt><dd>{p.email || '—'}</dd>
        <dt>User</dt><dd><code>{p.userId}</code></dd>
        <dt>Joined</dt><dd>{formatTime(p.joinedAt)}</dd>
        {p.gender && <><dt>Gender, age</dt><dd>{p.gender}{age !== null && `, about ${age}`}</dd></>}
        {p.homeArea && <><dt>Home</dt><dd>{p.homeArea} ({p.cityId})</dd></>}
        <dt>Reports</dt>
        <dd>
          {p.reportsAbout} about them ({p.openReportsAbout} open) · {p.reportsMade} made by them
          {p.reportsAbout > 0 && <> · <a href={`#/reports?about=${p.userId}`}>see reports about them</a></>}
        </dd>
      </dl>

      <Decision person={p} />

      <h2>Flags</h2>
      {p.flags.length === 0 ? <p className="muted">None.</p> : p.flags.map((f) => (
        <article key={f.id.toString()} className="card">
          <p>{f.kind === FlagKind.PICK_GENDER_LEAN ? leanSummary(f.detailsJson) : <code>{f.detailsJson}</code>}</p>
          <p className="meta">Raised {formatTime(f.createdAt)}{f.resolvedAt ? ` · resolved ${formatTime(f.resolvedAt)}${f.note ? `: ${f.note}` : ''}` : ' · open'}</p>
        </article>
      ))}

      <h2>Outings</h2>
      {p.outings.length === 0 ? <p className="muted">None yet.</p> : (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Date</th><th>Plan</th><th>Answer</th><th>Went?</th></tr></thead>
            <tbody>
              {p.outings.map((o) => (
                <tr key={o.eventId}>
                  <td className="nowrap">{o.date} {o.block}</td>
                  <td>{o.title} {o.eventStatus !== 'done' && <span className="muted">({o.eventStatus})</span>}</td>
                  <td>{o.response}</td>
                  <td>{[o.checkedIn && 'checked in', feedbackLabels[o.feedback]].filter(Boolean).join(', ')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <h2>Suspension history</h2>
      {p.suspensions.length === 0 ? <p className="muted">Never suspended.</p> : p.suspensions.map((s) => (
        <article key={s.id.toString()} className="card">
          <p className="meta">Suspended {formatTime(s.createdAt)}{s.liftedAt ? ` · lifted ${formatTime(s.liftedAt)}` : ' · in force'}</p>
          <blockquote>{s.reason}</blockquote>
          {s.liftNote && <p className="meta">Lifted: {s.liftNote}</p>}
        </article>
      ))}
    </section>
  );
}

/** Suspend (with a reason) or lift a suspension (with a note). */
function Decision({ person: p }: { person: PersonData }) {
  const queryClient = useQueryClient();
  const [text, setText] = useState('');
  const [confirming, setConfirming] = useState(false);
  const act = useMutation({
    mutationFn: async () => {
      if (p.suspended) await api.moderation.liftSuspension({ userId: p.userId, note: text });
      else await api.moderation.suspendPerson({ userId: p.userId, reason: text });
    },
    onSuccess: () => {
      setText('');
      setConfirming(false);
      void queryClient.invalidateQueries({ queryKey: ['person', p.userId] });
      void queryClient.invalidateQueries({ queryKey: ['suspended'] });
      void queryClient.invalidateQueries({ queryKey: ['people'] });
    },
  });

  return (
    <article className={p.suspended ? 'card decision' : 'card decision danger'}>
      {p.suspended ? (
        <>
          <h2>Lift the suspension</h2>
          <p className="muted">They can use Issho again, but aren't put back into the plans, outings or crews they were taken out of.</p>
          <label className="note">Note<textarea rows={2} maxLength={2000} value={text} onChange={(e) => setText(e.target.value)} placeholder="Why it's lifted" /></label>
          <div className="row"><button disabled={act.isPending} onClick={() => act.mutate()}>Lift suspension</button></div>
        </>
      ) : (
        <>
          <h2>Suspend</h2>
          <p className="muted">
            They're kept out of Issho: the app shows only that their account is suspended, matching leaves them out, and
            they get no notifications. They're taken out of their signups, outings that haven't started, and
            crews. The reason is for moderators; they aren't shown it.
          </p>
          <label className="note">Reason<textarea rows={2} maxLength={2000} value={text} onChange={(e) => setText(e.target.value)} placeholder="What happened" /></label>
          <div className="row">
            {!confirming ? (
              <button disabled={text.trim() === ''} onClick={() => setConfirming(true)}>Suspend…</button>
            ) : (
              <>
                <button disabled={act.isPending} onClick={() => act.mutate()}>Yes, suspend {p.name || 'them'}</button>
                <button className="secondary" onClick={() => setConfirming(false)}>Cancel</button>
              </>
            )}
          </div>
        </>
      )}
      {act.error && <p className="error">{ConnectError.from(act.error).message}</p>}
    </article>
  );
}
