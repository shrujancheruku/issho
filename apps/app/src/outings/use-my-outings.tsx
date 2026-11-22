import { InvitationAnswer, type Outing } from '@issho/api-client';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';

import { useApi } from '@/api/api-provider';
import { useAuth } from '@/auth/auth-provider';

import { feedbackOpen, feedbackRefusalOf, refusalOf, type FeedbackRead, type FeedbackRefusal, type OutingRefusal } from './model';

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
  /** "I'm here"; a refusal is in `refusals`. */
  checkIn: (outingId: string) => Promise<void>;
  /**
   * The person's feedback form for each outing whose feedback is open, read
   * once the outings are. It holds their own private picks, so it lives in
   * memory here and nowhere else: not in a key, a log or an address.
   */
  feedbacks: Record<string, FeedbackRead>;
  /** Reads an outing's feedback form again (after it could not be read). */
  loadFeedback: (outingId: string) => Promise<void>;
  /** Saves the person's answers, replacing the last; resolves to why it was refused, or null when it was saved. */
  saveFeedback: (outingId: string, answers: { went: boolean; notThere: string[]; meetAgain: string[] }) => Promise<FeedbackRefusal | null>;
};

const Context = createContext<Value | null>(null);
const NO_FORMS: Record<string, FeedbackRead> = {};

export function MyOutingsProvider({ children }: { children: ReactNode }) {
  const api = useApi();
  const auth = useAuth();
  const signedIn = auth.status === 'signedIn';
  const [read, setRead] = useState<MyOutings | null>(null);
  const [refusals, setRefusals] = useState<Record<string, OutingRefusal>>({});
  // The forms belong to whoever was signed in when they were read: another person signing in on this device never sees them.
  const owner = auth.status === 'signedIn' ? auth.session.email : null;
  const [store, setStore] = useState<{ owner: string | null; forms: Record<string, FeedbackRead> }>({ owner: null, forms: {} });
  const feedbacks = owner !== null && store.owner === owner ? store.forms : NO_FORMS;
  const setForm = useCallback((who: string | null, outingId: string, form: FeedbackRead) => setStore((s) => ({ owner: who, forms: s.owner === who ? { ...s.forms, [outingId]: form } : { [outingId]: form } })), []);
  // Outings whose form has been asked for (by whom), so a reload of the list does not ask again.
  const asked = useRef(new Set<string>());

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

  const loadFeedback = useCallback(
    async (outingId: string) => {
      asked.current.add(`${owner}:${outingId}`);
      setForm(owner, outingId, { status: 'loading' });
      try {
        const { feedback } = await api.outing.getFeedback({ outingId });
        setForm(owner, outingId, feedback ? { status: 'ready', feedback } : { status: 'failed' });
      } catch {
        setForm(owner, outingId, { status: 'failed' });
      }
    },
    [api, owner, setForm],
  );

  // Whether feedback was given is on the form, not the outing, so every outing whose feedback is open is asked for its form once the list is read.
  useEffect(() => {
    if (owner === null || read?.status !== 'ready') return;
    const now = new Date();
    for (const o of read.outings) if (feedbackOpen(o, now) && !asked.current.has(`${owner}:${o.id}`)) void loadFeedback(o.id);
  }, [owner, read, loadFeedback]);

  const saveFeedback = useCallback(
    async (outingId: string, answers: { went: boolean; notThere: string[]; meetAgain: string[] }) => {
      try {
        const { feedback } = await api.outing.saveFeedback({ outingId, ...answers });
        if (feedback) setForm(owner, outingId, { status: 'ready', feedback });
        return null;
      } catch (err) {
        // The windows are fixed once an outing is formed, so the list already held says which side of them the clock is on.
        return feedbackRefusalOf(err, read?.status === 'ready' ? read.outings.find((o) => o.id === outingId) : undefined, new Date());
      }
    },
    [api, owner, read, setForm],
  );

  // Every other outing's yes count and seat may have moved with an answer (becoming going declines the person's other open seats), so the whole list is read again.
  const act = useCallback(
    async (kind: 'answer' | 'cancel' | 'checkIn', outingId: string, call: () => Promise<unknown>) => {
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
  const checkIn = useCallback((outingId: string) => act('checkIn', outingId, () => api.outing.checkIn({ outingId })), [act, api]);

  const value = useMemo(() => {
    // Nothing is kept for a person who is not signed in (or has signed out since).
    const state: MyOutings = !signedIn ? { status: 'idle' } : (read ?? { status: 'loading' });
    return { state, refusals, reload, answer, cancel, checkIn, feedbacks, loadFeedback, saveFeedback };
  }, [signedIn, read, refusals, feedbacks, reload, answer, cancel, checkIn, loadFeedback, saveFeedback]);
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useMyOutings(): Value {
  const value = useContext(Context);
  if (!value) throw new Error('useMyOutings needs a MyOutingsProvider');
  return value;
}
