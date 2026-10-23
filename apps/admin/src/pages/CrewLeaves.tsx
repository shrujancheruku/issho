import { ConnectError, LeaveReason } from '@issho/api-client';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { formatTime, leaveReasons } from '../format';
import { api } from '../lib';

const reasons = [LeaveReason.UNSPECIFIED, LeaveReason.UNCOMFORTABLE, LeaveReason.NOT_INTERESTED, LeaveReason.INACTIVE, LeaveReason.TOO_MANY_MESSAGES, LeaveReason.OTHER];

export function CrewLeaves() {
  const [reason, setReason] = useState(LeaveReason.UNSPECIFIED);
  const leaves = useQuery({ queryKey: ['leaves', reason], queryFn: () => api.moderation.listCrewLeaves({ reason }) });

  return (
    <section>
      <div className="toolbar">
        <h1>Crew leaves</h1>
        <select value={reason} onChange={(e) => setReason(Number(e.target.value) as LeaveReason)}>
          {reasons.map((r) => <option key={r} value={r}>{r === LeaveReason.UNSPECIFIED ? 'Every reason' : leaveReasons[r]}</option>)}
        </select>
      </div>
      <p className="muted hint">
        Why people left crews; the crew isn't told. "Made me uncomfortable" may need a look at the crew's reports.
      </p>
      {leaves.isPending && <p className="muted">Loading…</p>}
      {leaves.error && <p className="error">{ConnectError.from(leaves.error).message}</p>}
      {leaves.data?.leaves.length === 0 && <p className="empty">Nothing here.</p>}
      {leaves.data && leaves.data.leaves.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>When</th><th>Crew</th><th>Who</th><th>Reason</th><th>Details</th></tr></thead>
            <tbody>
              {leaves.data.leaves.map((l) => (
                <tr key={l.id.toString()} className={l.reason === LeaveReason.UNCOMFORTABLE ? 'attention' : undefined}>
                  <td className="nowrap">{formatTime(l.createdAt)}</td>
                  <td>{l.crewName} <span className="muted">({l.crewStatus})</span></td>
                  <td>{l.userId ? l.userName || <span className="muted">No profile</span> : <span className="muted">Deleted account</span>}</td>
                  <td>{leaveReasons[l.reason]}</td>
                  <td>{l.details}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
