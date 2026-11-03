import { ConnectError } from '@issho/api-client';
import { useEffect, useState } from 'react';

import { formatTime, personHref } from '../format';
import { api } from '../lib';
import { LoadMore, rowsOf, usePaged } from '../paging';

export function People() {
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState(''); // search, applied after a pause in typing
  useEffect(() => {
    const t = setTimeout(() => setQuery(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);
  const found = usePaged(['people', query], (pageToken) => api.moderation.searchPeople({ query, pageToken }), query.length >= 2);
  const foundRows = rowsOf(found.data, (p) => p.people);
  const suspended = usePaged(['suspended'], (pageToken) => api.moderation.listSuspended({ pageToken }));
  const suspendedRows = rowsOf(suspended.data, (p) => p.people);

  return (
    <section>
      <div className="toolbar">
        <h1>People</h1>
        <input type="search" placeholder="Name, email or user id" value={search} onChange={(e) => setSearch(e.target.value)} autoFocus />
      </div>
      <p className="muted hint">Open someone to see their reports, flags and outings, and to suspend them.</p>
      {found.error && <p className="error">{ConnectError.from(found.error).message}</p>}
      {query.length >= 2 && found.data && foundRows.length === 0 && <p className="empty">Nobody matches.</p>}
      {foundRows.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Name</th><th>Email</th><th></th></tr></thead>
            <tbody>
              {foundRows.map((p) => (
                <tr key={p.userId}>
                  <td><a href={personHref(p.userId)}>{p.name || 'No profile'}</a></td>
                  <td>{p.email}</td>
                  <td>{p.suspended && <span className="badge suspended">Suspended</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <LoadMore query={found} />

      <h2>Suspended now</h2>
      {suspended.error && <p className="error">{ConnectError.from(suspended.error).message}</p>}
      {suspended.data && suspendedRows.length === 0 && <p className="empty">Nobody is suspended.</p>}
      {suspendedRows.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Name</th><th>Since</th><th>Why</th></tr></thead>
            <tbody>
              {suspendedRows.map((p) => (
                <tr key={p.userId}>
                  <td><a href={personHref(p.userId)}>{p.name || 'No profile'}</a></td>
                  <td className="nowrap">{formatTime(p.since)}</td>
                  <td>{p.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <LoadMore query={suspended} />
    </section>
  );
}
