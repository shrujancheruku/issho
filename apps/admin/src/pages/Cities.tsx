import { ConnectError, type CityPackSummary } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api } from '../lib';

/**
 * City packs: what's in the repository and the database, open pull requests
 * from the editor, and applying a merged pack (then finding venues).
 */
export function Cities() {
  const list = useQuery({ queryKey: ['city-packs'], queryFn: () => api.cityPacks.listCityPacks({}) });
  const data = list.data;
  return (
    <section>
      <div className="toolbar">
        <h1>Cities</h1>
        <span className="spacer" />
        {data?.configured && <a className="button" href="#/city/new">New city</a>}
      </div>
      <p className="muted hint">
        City packs live in the repository (<code>data/citypacks</code>). Editing one opens a pull request; once it's merged, apply it here to load it into the database.
      </p>
      {list.isPending && <p className="muted">Loading…</p>}
      {list.error && <p className="error">{ConnectError.from(list.error).message}</p>}
      {data && !data.configured && (
        <p className="error">The API has no GITHUB_TOKEN, so packs can't be read or proposed. See docs/secrets.md.</p>
      )}
      {data && data.pullRequests.length > 0 && (
        <div className="card">
          <h2>Waiting for review</h2>
          <ul>{data.pullRequests.map((p) => <li key={p.number}><a href={p.url} target="_blank" rel="noreferrer">#{p.number} {p.title}</a></li>)}</ul>
          <p className="muted hint">Merge on GitHub, then apply the city below.</p>
        </div>
      )}
      {data && (
        <div className="table-wrap">
          <table>
            <thead><tr><th>City</th><th>Repository</th><th>Database</th><th>Areas</th><th>Still needs Google</th><th /></tr></thead>
            <tbody>{data.packs.map((p) => <PackRow key={p.id} pack={p} configured={data.configured} />)}</tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function PackRow({ pack: p, configured }: { pack: CityPackSummary; configured: boolean }) {
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['city-packs'] });
  const apply = useMutation({ mutationFn: () => api.cityPacks.applyCityPack({ cityId: p.id }), onSuccess: refresh });
  const discover = useMutation({
    mutationFn: (areaId: string) => api.cityPacks.discoverAreaVenues({ cityId: p.id, areaId }),
    onSuccess: refresh,
  });
  const a = apply.data;
  const error = apply.error ?? discover.error;
  return (
    <tr>
      <td><strong>{p.name}</strong> <span className="muted">{p.id}</span></td>
      <td>{p.inRepo ? 'yes' : <span className="muted">no</span>}</td>
      <td>{p.seeded ? 'loaded' : <span className="muted">not loaded</span>}</td>
      <td>{p.areas || ''}</td>
      <td>
        {p.areasWithoutTravelTimes.length > 0 && <div>No travel times: {p.areasWithoutTravelTimes.join(', ')}</div>}
        {p.areasWithoutVenues.map((id) => (
          <div key={id}>
            No venues: {id}{' '}
            <button className="small secondary" disabled={discover.isPending} onClick={() => discover.mutate(id)}>Find venues</button>
          </div>
        ))}
        {discover.isPending && <span className="muted">Searching Google (one search per kind of place)…</span>}
        {discover.data && <span className="muted">{discover.data.venues} venues from {discover.data.searches} searches.</span>}
        {a && (
          <div className="muted">
            Applied{a.newAreas.length ? `; new areas ${a.newAreas.join(', ')}` : ''}{a.movedAreas.length ? `; moved ${a.movedAreas.join(', ')}` : ''}
            {a.travelTimesRebuilt ? `; ${a.travelTimes} travel times rebuilt` : ''}.
          </div>
        )}
        {error && <div className="error">{ConnectError.from(error).message}</div>}
      </td>
      <td className="nowrap">
        {configured && p.inRepo && <a className="button small secondary" href={`#/city/${p.id}`}>Edit</a>}{' '}
        {configured && p.inRepo && (
          <button className="small" disabled={apply.isPending} onClick={() => apply.mutate()} title="Load the pack on the default branch into the database">
            {apply.isPending ? 'Applying…' : 'Apply from main'}
          </button>
        )}
      </td>
    </tr>
  );
}
