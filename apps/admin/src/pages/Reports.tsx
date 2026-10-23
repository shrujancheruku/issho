import { ConnectError, ReportStatus, type ModerationReport } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { formatTime, personHref, reportReasons } from '../format';
import { api } from '../lib';

const filters: { label: string; status: ReportStatus }[] = [
  { label: 'Open', status: ReportStatus.OPEN },
  { label: 'Actioned', status: ReportStatus.ACTIONED },
  { label: 'Dismissed', status: ReportStatus.DISMISSED },
  { label: 'All', status: ReportStatus.UNSPECIFIED },
];

const statusLabels: Record<ReportStatus, string> = {
  [ReportStatus.UNSPECIFIED]: '',
  [ReportStatus.OPEN]: 'Open',
  [ReportStatus.ACTIONED]: 'Actioned',
  [ReportStatus.DISMISSED]: 'Dismissed',
};

/** about: only reports about this person (from their person view). */
export function Reports({ about }: { about?: string } = {}) {
  const [status, setStatus] = useState(about ? ReportStatus.UNSPECIFIED : ReportStatus.OPEN);
  // One person's reports, from "N reports" on a card or their person view.
  const [person, setPerson] = useState<{ id: string; name: string } | null>(about ? { id: about, name: '' } : null);
  const reports = useQuery({
    queryKey: ['reports', status, person?.id],
    queryFn: () => api.moderation.listReports({ status, reportedId: person?.id ?? '' }),
  });

  return (
    <section>
      <div className="toolbar">
        <h1>Reports</h1>
        <div className="tabs">
          {filters.map((f) => (
            <button key={f.label} className={f.status === status ? 'tab active' : 'tab'} onClick={() => setStatus(f.status)}>{f.label}</button>
          ))}
        </div>
        {person && (
          <span className="chip">
            About {person.name || reports.data?.reports[0]?.reportedName || 'someone'} <button className="link" onClick={() => setPerson(null)}>× everyone</button>
          </span>
        )}
      </div>
      <p className="muted hint">
        A report already keeps the two people out of each other's groups and crews, whatever is decided here.
      </p>
      {reports.isPending && <p className="muted">Loading…</p>}
      {reports.error && <p className="error">{ConnectError.from(reports.error).message}</p>}
      {reports.data?.reports.length === 0 && <p className="empty">Nothing here.</p>}
      {reports.data?.reports.map((r) => (
        <ReportCard key={r.id} report={r} onPerson={() => { setPerson({ id: r.reportedId, name: r.reportedName }); setStatus(ReportStatus.UNSPECIFIED); }} />
      ))}
    </section>
  );
}

function ReportCard({ report: r, onPerson }: { report: ModerationReport; onPerson: () => void }) {
  const queryClient = useQueryClient();
  const [note, setNote] = useState(r.note);
  const resolve = useMutation({
    mutationFn: (status: ReportStatus) => api.moderation.resolveReport({ reportId: r.id, status, note }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['reports'] }),
  });
  const where = r.eventId
    ? `Outing: ${r.eventTitle} · ${r.eventDate} ${r.eventBlock}`
    : r.crewId ? `Crew: ${r.crewName}` : 'Outing or crew no longer exists';

  return (
    <article className="card">
      <div className="card-head">
        <span className={`badge reason-${r.reason}`}>{reportReasons[r.reason]}</span>
        <a href={personHref(r.reportedId)}><strong>{r.reportedName || 'No profile'}</strong></a>
        <button className="link" onClick={onPerson}>
          {r.reportsAboutReported} {r.reportsAboutReported === 1 ? 'report' : 'reports'} in all
        </button>
        <span className="spacer" />
        <span className={`status status-${statusLabels[r.status].toLowerCase()}`}>{statusLabels[r.status]}</span>
      </div>
      <p className="meta">
        Reported by {r.reporterId ? <a href={personHref(r.reporterId)}>{r.reporterName || 'someone with no profile'}</a> : 'a deleted account'} · {formatTime(r.createdAt)}
        {r.updatedAt && formatTime(r.updatedAt) !== formatTime(r.createdAt) && ` · last changed ${formatTime(r.updatedAt)}`}
      </p>
      <p className="meta">{where}</p>
      {r.details ? <blockquote>{r.details}</blockquote> : <p className="muted">No details given.</p>}
      <dl className="ids">
        <dt>Reported</dt><dd><code>{r.reportedId}</code></dd>
        {r.reporterId && <><dt>Reporter</dt><dd><code>{r.reporterId}</code></dd></>}
      </dl>
      {r.reviewedAt && (
        <p className="meta">
          Reviewed {formatTime(r.reviewedAt)}
          {r.updatedAt && r.updatedAt.seconds > r.reviewedAt.seconds && <strong className="error"> · the reporter changed it since</strong>}
        </p>
      )}
      <label className="note">
        Moderator note
        <textarea rows={2} maxLength={2000} value={note} onChange={(e) => setNote(e.target.value)} placeholder="What you did and why" />
      </label>
      <div className="row">
        {r.status !== ReportStatus.ACTIONED && <button disabled={resolve.isPending} onClick={() => resolve.mutate(ReportStatus.ACTIONED)}>Mark actioned</button>}
        {r.status !== ReportStatus.DISMISSED && <button className="secondary" disabled={resolve.isPending} onClick={() => resolve.mutate(ReportStatus.DISMISSED)}>Dismiss</button>}
        {r.status !== ReportStatus.OPEN && <button className="secondary" disabled={resolve.isPending} onClick={() => resolve.mutate(ReportStatus.OPEN)}>Reopen</button>}
        {r.status !== ReportStatus.OPEN && note !== r.note && <button className="secondary" disabled={resolve.isPending} onClick={() => resolve.mutate(r.status)}>Save note</button>}
      </div>
      {resolve.error && <p className="error">{ConnectError.from(resolve.error).message}</p>}
    </article>
  );
}
