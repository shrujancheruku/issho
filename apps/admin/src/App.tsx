import { Code, ConnectError } from '@issho/api-client';
import type { Session } from '@supabase/supabase-js';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { api, supabase } from './lib';
import { CrewLeaves } from './pages/CrewLeaves';
import { Flags } from './pages/Flags';
import { Reports } from './pages/Reports';
import { Translations } from './pages/Translations';
import { SignIn } from './SignIn';

const pages = {
  reports: { label: 'Reports', component: Reports },
  flags: { label: 'Flags', component: Flags },
  leaves: { label: 'Crew leaves', component: CrewLeaves },
  translations: { label: 'Translations', component: Translations },
} as const;
type Page = keyof typeof pages;

function pageFromHash(): Page {
  const h = window.location.hash.replace(/^#\/?/, '');
  return h in pages ? (h as Page) : 'reports';
}

export function App() {
  // undefined while the stored session loads, null when signed out.
  const [session, setSession] = useState<Session | null | undefined>(undefined);
  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => setSession(data.session));
    const { data } = supabase.auth.onAuthStateChange((_event, s) => setSession(s));
    return () => data.subscription.unsubscribe();
  }, []);

  if (session === undefined) return null;
  if (!session) return <SignIn />;
  return <Console />;
}

function Console() {
  const queryClient = useQueryClient();
  const me = useQuery({ queryKey: ['moderator'], queryFn: () => api.moderation.getModerator({}), retry: false });
  const [page, setPage] = useState<Page>(pageFromHash);
  useEffect(() => {
    const onHash = () => setPage(pageFromHash());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  const signOut = async () => {
    await supabase.auth.signOut();
    queryClient.clear();
  };

  if (me.isPending) return <main className="center muted">Checking your account…</main>;
  if (me.error) {
    const denied = ConnectError.from(me.error).code === Code.PermissionDenied;
    return (
      <main className="center">
        <div className="card narrow">
          <h1>{denied ? 'Not an admin' : 'Something went wrong'}</h1>
          <p className="muted">
            {denied
              ? 'This account has no admin role. An admin can grant it with `tool set-admin`; then check again.'
              : ConnectError.from(me.error).message}
          </p>
          <div className="row">
            {/* The role is read from the access token: a fresh one picks up a new grant. */}
            <button onClick={async () => { await supabase.auth.refreshSession(); me.refetch(); }}>Check again</button>
            <button className="secondary" onClick={signOut}>Sign out</button>
          </div>
        </div>
      </main>
    );
  }

  const Current = pages[page].component;
  return (
    <div className="shell">
      <header className="topbar">
        <span className="brand">ISSHO <span className="muted">admin</span></span>
        <nav>
          {(Object.keys(pages) as Page[]).map((p) => (
            <a key={p} href={`#/${p}`} className={p === page ? 'active' : undefined}>{pages[p].label}</a>
          ))}
        </nav>
        <span className="who muted">{me.data.email}</span>
        <button className="secondary small" onClick={signOut}>Sign out</button>
      </header>
      <main className="content">
        <Current />
      </main>
    </div>
  );
}
