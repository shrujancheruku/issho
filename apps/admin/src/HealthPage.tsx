import { useEffect, useState } from 'react';

import type { Api } from '@issho/api-client';

type Status = { kind: 'checking' } | { kind: 'ok'; version: string } | { kind: 'failed' };

// The admin's one page for now: it asks the public health check which build
// of the API it is talking to. Later milestones add the screens around it.
export function HealthPage({ api }: { api: Api }) {
  const [status, setStatus] = useState<Status>({ kind: 'checking' });

  useEffect(() => {
    let current = true;
    api.health
      .check({})
      .then((res) => current && setStatus({ kind: 'ok', version: res.version }))
      .catch(() => current && setStatus({ kind: 'failed' }));
    // A stale response from an unmounted page must not overwrite state.
    return () => {
      current = false;
    };
  }, [api]);

  if (status.kind === 'checking') return <p>Checking the API…</p>;
  if (status.kind === 'failed') return <p>The API could not be reached.</p>;
  return <p>API version {status.version}</p>;
}
