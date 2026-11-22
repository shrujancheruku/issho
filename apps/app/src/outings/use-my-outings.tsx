import { InvitationAnswer, type Outing } from '@issho/api-client';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { useApi } from '@/api/api-provider';
import { useAuth } from '@/auth/auth-provider';

import { refusalOf, type OutingRefusal } from './model';

/**
 * The signed-in person's own outings (ListMyOutings). Like the signups they load
 * on the client only, for a signed-in person only: a plan's page is
 * server-rendered and publicly cached, so nothing per visitor is in it. One
 * copy is shared by the tab's badge, Together and a plan's strip, so an answer
 * in one is seen by the others.
 */
export type MyOutings = { status: 'idle' } | { status: 'loading' } | { status: 'failed' } | { status: 'ready'; outings: Outing[] };

type Value = {
  state: MyOutings;
  /** Why the last answer or cancel of each outing was refused, until the next try: it stays on the outing's card, row or strip even when the reload moves the outing elsewhere. */
  refusals: Record<string, OutingRefusal>;
  reload: () => Promise<void>;
  /** Answers an invitation; a refusal is in `refusals`. */
  answer: (outingId: string, yes: boolean) => Promise<void>;
  /** Cancels the caller's seat in an outing they are going to. */
  cancel: (outingId: string) => Promise<void>;
};

const Context = createContext<Value | null>(null);

export function MyOutingsProvider({ children }: { children: ReactNode }) {
  const api = useApi();
  const auth = useAuth();
  const signedIn = auth.status === 'signedIn';
  const [read, setRead] = useState<MyOutings | null>(null);
  const [refusals, setRefusals] = useState<Record<string, OutingRefusal>>({});

  const fetchOutings = useCallback(async (): Promise<Outing[] | null> => {
    try {
      return (await api.outing.listMyOutings({})).outings;
    } catch {
      return null;
    }
  }, [api]);

  const reload = useCallback(async () => {
    const outings = await fetchOutings();
    setRead(outings ? { status: 'ready', outings } : { status: 'failed' });
  }, [fetchOutings]);

  useEffect(() => {
    if (!signedIn) return;
    let live = true;
    fetchOutings().then((outings) => live && setRead(outings ? { status: 'ready', outings } : { status: 'failed' }));
    return () => {
      live = false;
    };
  }, [signedIn, fetchOutings]);

  // Every other outing's yes count and seat may have moved with an answer (becoming going declines the person's other open seats), so the whole list is read again.
  const act = useCallback(
    async (kind: 'answer' | 'cancel', outingId: string, call: () => Promise<unknown>) => {
      const refuse = (refusal: OutingRefusal | null) =>
        setRefusals(({ [outingId]: _gone, ...rest }) => (refusal ? { ...rest, [outingId]: refusal } : rest));
      refuse(null);
      try {
        await call();
        await reload();
      } catch (err) {
        const outings = await fetchOutings();
        if (outings) setRead({ status: 'ready', outings });
        refuse(refusalOf(err, kind, outings?.find((o) => o.id === outingId), new Date()));
      }
    },
    [fetchOutings, reload],
  );

  const answer = useCallback(
    (outingId: string, yes: boolean) =>
      act('answer', outingId, () => api.outing.answerInvitation({ outingId, answer: yes ? InvitationAnswer.YES : InvitationAnswer.NO })),
    [act, api],
  );
  const cancel = useCallback((outingId: string) => act('cancel', outingId, () => api.outing.cancelSeat({ outingId })), [act, api]);

  const value = useMemo(() => {
    // Nothing is kept for a person who is not signed in (or has signed out since).
    const state: MyOutings = !signedIn ? { status: 'idle' } : (read ?? { status: 'loading' });
    return { state, refusals, reload, answer, cancel };
  }, [signedIn, read, refusals, reload, answer, cancel]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useMyOutings(): Value {
  const value = useContext(Context);
  if (!value) throw new Error('useMyOutings needs a MyOutingsProvider');
  return value;
}
