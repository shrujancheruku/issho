import { ConnectError, ReviewEventStatus, type ReviewEvent, type ReviewGroup } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { localParts, localTime, personHref } from '../format';
import { api } from '../lib';

/** One proposed round (or scheduled event): its time, groups and people. */
export function GroupReview({ eventId }: { eventId: string }) {
  const queryClient = useQueryClient();
  const res = useQuery({ queryKey: ['review-event', eventId], queryFn: () => api.review.getReviewEvent({ eventId }) });
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['review-event', eventId] });
    void queryClient.invalidateQueries({ queryKey: ['review-events'] });
  };
  const act = useMutation({
    mutationFn: async (fn: () => Promise<unknown>) => { await fn(); },
    onSuccess: refresh,
  });
  if (res.isPending) return <p className="muted">Loading…</p>;
  if (res.error) return <p className="error">{ConnectError.from(res.error).message}</p>;
  const e = res.data.event!;
  const groups = res.data.groups;
  const proposed = e.status === ReviewEventStatus.PROPOSED;

  return (
    <section>
      <p><a href="#/groups">← Groups</a></p>
      <div className="toolbar">
        <h1>{e.title}</h1>
        <span className={`badge ${proposed ? 'status-draft' : 'status-open'}`}>{proposed ? 'Proposed' : 'Scheduled'}</span>
      </div>
      <dl className="ids facts">
        <dt>Where</dt><dd>{e.areaName}</dd>
        <dt>Starts</dt><dd>{localTime(e.startsAt, e.timeZone)} ({e.block}, {e.timeZone})</dd>
        <dt>Answers by</dt><dd>{localTime(e.confirmBy, e.timeZone)}</dd>
        <dt>Plan</dt><dd><a href={`#/plan/${e.planId}`}>open the plan</a>{e.forced && ' · scheduled at the signup deadline with whoever could go'}</dd>
      </dl>

      {proposed && <StartTime event={e} onSave={(date, time) => act.mutate(() => api.review.setProposalStart({ eventId, date, time }))} busy={act.isPending} />}

      {groups.map((g, i) => (
        <GroupCard key={g.id} index={i} group={g} groups={groups} proposed={proposed} busy={act.isPending}
          onMove={(userId, groupId) => act.mutate(() => api.review.moveProposalMember({ eventId, userId, groupId }))}
          onRemove={(userId) => act.mutate(() => api.review.removeProposalMember({ eventId, userId }))}
          onMeeting={(text) => act.mutate(() => api.review.setGroupMeetingPoint({ groupId: g.id, text }))} />
      ))}

      {proposed && (
        <article className="card decision">
          <div className="row">
            <button disabled={act.isPending} onClick={() => act.mutate(() => api.review.approveProposal({ eventId }))}>Approve and send invitations</button>
            <button className="secondary" disabled={act.isPending} onClick={() => act.mutate(() => api.review.rejectProposal({ eventId }))}>Reject</button>
          </div>
          <p className="muted hint">
            Approving invites everyone still signed up and tells them; anyone who withdrew since is left out. Rejecting
            keeps everyone signed up, and the plan isn't scheduled again for a day.
          </p>
        </article>
      )}
      {act.error && <p className="error">{ConnectError.from(act.error).message}</p>}
      {act.isSuccess && !proposed && <p className="muted">Saved.</p>}
    </section>
  );
}

function StartTime({ event: e, onSave, busy }: { event: ReviewEvent; onSave: (date: string, time: string) => void; busy: boolean }) {
  const start = localParts(e.startsAt, e.timeZone);
  const [date, setDate] = useState(start.date || e.date);
  const [time, setTime] = useState(start.time);
  return (
    <article className="card">
      <div className="row">
        <label>Date<input type="date" value={date} onChange={(ev) => setDate(ev.target.value)} /></label>
        <label>Start ({e.timeZone})<input type="time" value={time} onChange={(ev) => setTime(ev.target.value)} /></label>
        <button className="secondary" disabled={busy || (date === start.date && time === start.time)} onClick={() => onSave(date, time)}>Change start</button>
      </div>
      <p className="muted hint">People answer by the start of the day before. The scheduler chose this from when people said they're free; check before moving it.</p>
    </article>
  );
}

function GroupCard({ index, group: g, groups, proposed, busy, onMove, onRemove, onMeeting }: {
  index: number; group: ReviewGroup; groups: ReviewGroup[]; proposed: boolean; busy: boolean;
  onMove: (userId: string, groupId: string) => void; onRemove: (userId: string) => void; onMeeting: (text: string) => void;
}) {
  const [meeting, setMeeting] = useState(g.meetingPoint);
  const year = new Date().getFullYear();
  const short = g.members.length < g.minAttendance;
  return (
    <article className={short ? 'card group danger' : 'card group'}>
      <div className="card-head">
        <strong>Group {index + 1}</strong>
        <span className="muted">{g.members.length} people · confirms at {g.quorum}, goes ahead with {g.minAttendance} · score {g.score.toFixed(2)}{!proposed && ` · ${g.status}`}</span>
      </div>
      {short && <p className="error">Fewer than its minimum: move someone in or it can't be approved.</p>}
      <table>
        <tbody>
          {g.members.map((m) => (
            <tr key={m.userId}>
              <td><a href={personHref(m.userId)}>{m.name || 'No profile'}</a>{!m.signedUp && proposed && <span className="badge small-badge">withdrew</span>}</td>
              <td>{m.gender}{m.birthYear > 0 && `, ${year - m.birthYear}`}</td>
              <td>{proposed ? '' : m.response}</td>
              {proposed && (
                <td className="nowrap">
                  {groups.length > 1 && (
                    <select value="" disabled={busy} onChange={(ev) => ev.target.value && onMove(m.userId, ev.target.value)}>
                      <option value="">Move to…</option>
                      {groups.map((o, j) => o.id !== g.id && <option key={o.id} value={o.id}>Group {j + 1}</option>)}
                    </select>
                  )}
                  <button className="link" disabled={busy} onClick={() => onRemove(m.userId)}>take out</button>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
      <div className="row meeting">
        <label className="note">Meeting point <span className="muted">(English; e.g. "North exit, by the clock")</span>
          <input maxLength={300} value={meeting} onChange={(ev) => setMeeting(ev.target.value)} />
        </label>
        <button className="secondary" disabled={busy || meeting === g.meetingPoint} onClick={() => onMeeting(meeting)}>Save</button>
      </div>
    </article>
  );
}
