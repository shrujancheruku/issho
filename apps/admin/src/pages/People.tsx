import { ConnectError } from '@issho/api-client';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { formatTime, personHref } from '../format';
import { api } from '../lib';

export function People() {
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState(''); // search, applied after a pause in typing
  useEffect(() => {
    const t = setTimeout(() => setQuery(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);
  const found = useQuery({
    queryKey: ['people', query],
    queryFn: () => api.moderation.searchPeople({ query }),
    enabled: query.length >= 2,
  });
  const suspended = useQuery({ queryKey: ['suspended'], queryFn: () => api.moderation.listSuspended({}) });

  return (
    <section>
      <div className="toolbar">
        <h1>People</h1>
        <input type="search" placeholder="Name, email or user id" value={search} onChange={(e) => setSearch(e.target.value)} autoFocus />
      </div>
      <p className="muted hint">Open someone to see their reports, flags and outings, and to suspend them.</p>
      {found.error && <p className="error">{ConnectError.from(found.error).message}</p>}
      {query.length >= 2 && found.data?.people.length === 0 && <p className="empty">Nobody matches.</p>}
      {found.data && found.data.people.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Name</th><th>Email</th><th></th></tr></thead>
            <tbody>
              {found.data.people.map((p) => (
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

      <h2>Suspended now</h2>
      {suspended.error && <p className="error">{ConnectError.from(suspended.error).message}</p>}
      {suspended.data?.people.length === 0 && <p className="empty">Nobody is suspended.</p>}
      {suspended.data && suspended.data.people.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Name</th><th>Since</th><th>Why</th></tr></thead>
            <tbody>
              {suspended.data.people.map((p) => (
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
    </section>
  );
}
