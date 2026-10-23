import { ConnectError, FlagKind, type ModerationFlag } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { formatTime, leanSummary, personHref } from '../format';
import { api } from '../lib';

export function Flags() {
  const [resolved, setResolved] = useState(false);
  const flags = useQuery({ queryKey: ['flags', resolved], queryFn: () => api.moderation.listFlags({ resolved }) });

  return (
    <section>
      <div className="toolbar">
        <h1>Flags</h1>
        <div className="tabs">
          <button className={!resolved ? 'tab active' : 'tab'} onClick={() => setResolved(false)}>Open</button>
          <button className={resolved ? 'tab active' : 'tab'} onClick={() => setResolved(true)}>Resolved</button>
        </div>
      </div>
      <p className="muted hint">
        Signals from the system for a person to look at. A flag changes nothing in matching by itself.
      </p>
      {flags.isPending && <p className="muted">Loading…</p>}
      {flags.error && <p className="error">{ConnectError.from(flags.error).message}</p>}
      {flags.data?.flags.length === 0 && <p className="empty">Nothing here.</p>}
      {flags.data?.flags.map((f) => <FlagCard key={f.id.toString()} flag={f} />)}
    </section>
  );
}

function FlagCard({ flag: f }: { flag: ModerationFlag }) {
  const queryClient = useQueryClient();
  const [note, setNote] = useState('');
  const resolve = useMutation({
    mutationFn: () => api.moderation.resolveFlag({ flagId: f.id, note }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['flags'] }),
  });
  const lean = f.kind === FlagKind.PICK_GENDER_LEAN;

  return (
    <article className="card">
      <div className="card-head">
        <span className="badge">{lean ? 'Picks lean to one gender' : 'Flag'}</span>
        <a href={personHref(f.userId)}><strong>{f.userName || 'No profile'}</strong></a>
        <span className="spacer" />
        <span className="meta">Raised {formatTime(f.createdAt)}{formatTime(f.updatedAt) !== formatTime(f.createdAt) && ` · updated ${formatTime(f.updatedAt)}`}</span>
      </div>
      <p>{lean ? leanSummary(f.detailsJson) : <code>{f.detailsJson}</code>}</p>
      {lean && (
        <p className="muted hint">
          Their groups are mixed, so this compares picks with who they actually met. It's a statistical signal,
          not proof: look for other reports about them before acting.
        </p>
      )}
      <dl className="ids"><dt>User</dt><dd><code>{f.userId}</code></dd></dl>
      {f.resolvedAt ? (
        <p className="meta">Resolved {formatTime(f.resolvedAt)}{f.note && `: ${f.note}`}</p>
      ) : (
        <>
          <label className="note">
            Moderator note
            <textarea rows={2} maxLength={2000} value={note} onChange={(e) => setNote(e.target.value)} placeholder="What you found" />
          </label>
          <div className="row"><button disabled={resolve.isPending} onClick={() => resolve.mutate()}>Resolve</button></div>
        </>
      )}
      {resolve.error && <p className="error">{ConnectError.from(resolve.error).message}</p>}
    </article>
  );
}
