import {
  AdminJobState, Code, ConnectError, type ApplyCityPackResponse, type CityPackSummary, type DiscoverAreaVenuesResponse,
  type GetAdminJobResponse,
} from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

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

/** How often a running job is asked about (the API's AdminService comment says 2 s too). */
export const JOB_POLL_MS = 2000;

const finished = (j?: GetAdminJobResponse) => j?.state === AdminJobState.SUCCEEDED || j?.state === AdminJobState.FAILED;

/**
 * Follows an admin job (apply, find venues) until it succeeds or fails,
 * asking every JOB_POLL_MS. `jobId` is undefined before the job is queued and
 * 0n from an API that still ran the operation inside the request.
 */
/** Failed polls in a row before the page gives up and shows the error. */
export const POLL_FAILURES_SHOWN = 3;

class FailedPolls {
  failed = 0;
  ok() { this.failed = 0; }
  fail() { this.failed += 1; }
}

function useAdminJob(jobId: bigint | undefined) {
  // Consecutive failed polls (react-query's own failureCount restarts with
  // every fetch). The query re-renders after each failure, so reading it then
  // is current.
  const [polls] = useState(() => new FailedPolls());
  const query = useQuery({
    queryKey: ['admin-job', String(jobId)],
    queryFn: async () => {
      try {
        const j = await api.admin.getAdminJob({ jobId: jobId! });
        polls.ok();
        return j;
      } catch (e) {
        polls.fail();
        throw e;
      }
    },
    enabled: !!jobId,
    // Stop when it is done, or gone (River clears finished jobs after a day
    // or a week); keep asking through a failed poll, which is usually a blip.
    refetchInterval: (q) => (finished(q.state.data) || (q.state.error && ConnectError.from(q.state.error).code === Code.NotFound) ? false : JOB_POLL_MS),
  });
  // A job that is gone (NotFound) is final at once; any other error counts
  // only once it persists and until then the job is still running.
  const gone = !!query.error && ConnectError.from(query.error).code === Code.NotFound;
  const persistent = gone || (!!query.error && polls.failed >= POLL_FAILURES_SHOWN);
  return { ...query, shownError: persistent ? query.error : null };
}

/**
 * One long operation's progress. `started` is what the start RPC returned:
 * the job's id, or (from the previous API release, which ran the operation in
 * the request) the finished result itself, which is then shown as it was.
 */
function progress<R extends ApplyCityPackResponse | DiscoverAreaVenuesResponse>(
  starting: boolean, started: R | undefined, job: ReturnType<typeof useAdminJob>, pick: (j: GetAdminJobResponse) => R | undefined,
) {
  const queued = !!started && started.jobId !== 0n;
  const result = queued ? (job.data ? pick(job.data) : undefined) : started;
  return {
    result,
    // Starting the job, or waiting for it to end.
    running: starting || (queued && !finished(job.data) && !job.shownError),
    failure: job.data?.state === AdminJobState.FAILED ? (job.data.failure?.message ?? 'The job failed.') : undefined,
    error: job.shownError,
  };
}

const count = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

function PackRow({ pack: p, configured }: { pack: CityPackSummary; configured: boolean }) {
  const queryClient = useQueryClient();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['city-packs'] });
  // A queued job (job id set) refreshes the list when it ends; an API that
  // answered in the request (job id 0) is done at once.
  const refreshIfDone = (r: { jobId: bigint }) => (r.jobId === 0n ? refresh() : undefined);
  const apply = useMutation({ mutationFn: () => api.cityPacks.applyCityPack({ cityId: p.id }), onSuccess: refreshIfDone });
  const discover = useMutation({
    mutationFn: (areaId: string) => api.cityPacks.discoverAreaVenues({ cityId: p.id, areaId }),
    onSuccess: refreshIfDone,
  });
  const applyJob = useAdminJob(apply.data?.jobId);
  const discoverJob = useAdminJob(discover.data?.jobId);
  const applying = progress(apply.isPending, apply.data, applyJob, (j) => (j.result.case === 'applyCityPack' ? j.result.value : undefined));
  const finding = progress(discover.isPending, discover.data, discoverJob, (j) => (j.result.case === 'discoverAreaVenues' ? j.result.value : undefined));
  const a = applying.running ? undefined : applying.result;
  const venues = finding.result;
  const error = apply.error ?? discover.error ?? applying.error ?? finding.error;
  const failure = applying.failure ?? finding.failure;
  // A finished job changes what the list shows (areas, venues, travel times).
  const applyDone = finished(applyJob.data);
  const discoverDone = finished(discoverJob.data);
  useEffect(() => {
    if (applyDone) void queryClient.invalidateQueries({ queryKey: ['city-packs'] });
  }, [applyDone, queryClient]);
  useEffect(() => {
    if (discoverDone) void queryClient.invalidateQueries({ queryKey: ['city-packs'] });
  }, [discoverDone, queryClient]);
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
            <button className="small secondary" disabled={finding.running || applying.running} onClick={() => discover.mutate(id)}>Find venues</button>
          </div>
        ))}
        {finding.running && <span className="muted">Running… searching Google (one search per kind of place)</span>}
        {venues && !finding.running && <span className="muted">{venues.venues} venues from {venues.searches} searches.</span>}
        {a && (
          <div className="muted">
            Applied{a.newAreas.length ? `; new areas ${a.newAreas.join(', ')}` : ''}{a.movedAreas.length ? `; moved ${a.movedAreas.join(', ')}` : ''}
            {a.travelTimesRebuilt ? `; ${a.travelTimes} travel times rebuilt` : ''}.
          </div>
        )}
        {/* An authoritative apply deletes what the pack no longer lists; say so. */}
        {a?.removedAreas.length ? <div className="muted">Removed areas: {a.removedAreas.join(', ')}.</div> : null}
        {a?.replacedAreas.map((r) => (
          <div key={r.oldAreaId} className="muted">
            Replaced {r.oldAreaId} with {r.newAreaId}: moved {count(r.people, 'person', 'people')}, {count(r.crews, 'crew')}, {count(r.plans, 'plan')}.
          </div>
        ))}
        {a?.removedOverrides.length ? <div className="muted">Removed activity overrides: {a.removedOverrides.join(', ')}.</div> : null}
        {applying.running && <div className="muted">Running… applying the pack (rebuilding travel times can take a few minutes)</div>}
        {failure && <div className="error">Failed: {failure}</div>}
        {error && <div className="error">{ConnectError.from(error).message}</div>}
      </td>
      <td className="nowrap">
        {configured && p.inRepo && <a className="button small secondary" href={`#/city/${p.id}`}>Edit</a>}{' '}
        {configured && p.inRepo && (
          <button className="small" disabled={applying.running || finding.running} onClick={() => apply.mutate()} title="Load the pack on the default branch into the database">
            {applying.running ? 'Running…' : 'Apply from main'}
          </button>
        )}
      </td>
    </tr>
  );
}
