import { Code, ConnectError } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useSyncExternalStore } from 'react';

import { api, getAccessToken, wsBaseUrl } from '@/lib/api';
import { keys, roomId, type RoomRef } from '@/lib/queries';
import { ms } from '@/lib/time';

import { fromApi, merge, parseFrame, type Message } from './messages';

export type { RoomRef };

/** A room as the API's `ChatRoom` has it. */
const roomOf = (room: RoomRef) => ({
  room: { room: 'eventId' in room ? { case: 'eventId' as const, value: room.eventId } : { case: 'crewId' as const, value: room.crewId } },
});

const roomFromId = (id: string): RoomRef => (id.startsWith('event:') ? { eventId: id.slice(6) } : { crewId: id.slice(5) });

/** What the cache holds for a room: its messages and what the server last said about writing. */
export type RoomData = {
  messages: Message[];
  /** The server's decision: may write now. */
  open: boolean;
  /** When writing opens and closes (ms); unset when the room has no schedule. */
  opensAt?: number;
  closesAt?: number;
  /** Not open, and the opening is still to come (the window has not ended). */
  notYet: boolean;
};

export type RoomStatus = 'loading' | 'error' | 'notMember' | 'notOpen' | 'open' | 'closed';

const lastId = (data?: RoomData) => (data?.messages.length ? data.messages[data.messages.length - 1].id : 0);

/**
 * Reads the room: the latest messages the first time, then only what's after
 * the newest message the cache has, so a refetch (a room opening, the retry
 * button) keeps what the socket brought.
 */
async function fetchRoom(qc: QueryClient, room: RoomRef): Promise<RoomData> {
  const prev = qc.getQueryData<RoomData>(keys.room(room));
  const res = await api.chat.listMessages({ ...roomOf(room), afterId: BigInt(lastId(prev)) });
  const opensAt = ms(res.opensAt);
  const closesAt = ms(res.closesAt);
  return {
    messages: merge(prev?.messages ?? [], res.messages.map(fromApi)),
    open: res.open,
    opensAt: Number.isNaN(opensAt) ? undefined : opensAt,
    closesAt: Number.isNaN(closesAt) ? undefined : closesAt,
    // Not open with the window not over: still waiting for it to open, even
    // if the device clock is past opensAt (it can run ahead of the server's).
    notYet: !res.open && !Number.isNaN(opensAt) && !(closesAt <= Date.now()),
  };
}

/** The API says the person isn't in the room (or there is none), which asking again won't change. */
const denied = (e: unknown) => e instanceof ConnectError && (e.code === Code.PermissionDenied || e.code === Code.NotFound);

/** How long until the room's next opening or closing, if it is still ahead (ms from `now`). */
export function untilNextBoundary(data: RoomData | undefined, now: number): number | undefined {
  const ahead = [data?.opensAt, data?.closesAt].filter((t): t is number => t !== undefined && t > now);
  // A second late, so the server's clock has passed it too; and setTimeout can't wait longer than 2^31 ms.
  return ahead.length ? Math.min(Math.min(...ahead) - now + 1000, 2 ** 31 - 1) : undefined;
}

// Waits (ms) before asking again when a boundary refetch shows no change.
const skewRetryDelays = [15_000, 30_000, 60_000];

// Close codes from the server (backend internal/chatws) after which
// reconnecting can't help: not allowed in the room, or no such room.
const permanent = new Set([4403, 4404]);

// Typing: tell the others at most this often while the person types, and
// show someone as typing for this long after their last notice (their
// app re-sends every couple of seconds while they keep typing).
const typingEvery = 2000;
const typingShown = 5000;

type Typers = Readonly<Record<string, { name: string; until: number }>>;
const nobody: Typers = {};

// Who's typing in each room, and who wants to know. Kept outside the socket
// so a screen can subscribe whether or not the socket exists yet.
const typersByRoom = new Map<string, Typers>();
const typingListeners = new Map<string, Set<() => void>>();

function publishTypers(id: string, next: Typers) {
  if (Object.keys(next).length) typersByRoom.set(id, next);
  else typersByRoom.delete(id);
  typingListeners.get(id)?.forEach((l) => l());
}

function subscribeTyping(id: string, listener: () => void) {
  const set = typingListeners.get(id) ?? new Set();
  typingListeners.set(id, set.add(listener));
  return () => {
    set.delete(listener);
    if (!set.size) typingListeners.delete(id);
  };
}

/**
 * One socket on one room, shared by everyone showing that room. It writes
 * what it receives into the room's cache entry, reconnects with backoff
 * asking for everything after the newest message the cache has, and keeps
 * who's typing up to date for its consumers.
 */
class RoomSocket {
  refs = 0;
  private id: string;
  private ws: WebSocket | null = null;
  private stopped = false;
  private attempt = 0;
  private reconnect: ReturnType<typeof setTimeout> | undefined;
  private expire: ReturnType<typeof setTimeout> | undefined;
  // Whether the server has said it takes typing notices ("ready"), and when
  // this socket last sent one.
  private canType = false;
  private lastTyped = 0;

  constructor(
    private qc: QueryClient,
    private room: RoomRef,
  ) {
    this.id = roomId(room);
    void this.connect();
  }

  private get typers() {
    return typersByRoom.get(this.id) ?? nobody;
  }

  private setTypers(next: Typers) {
    clearTimeout(this.expire);
    const until = Object.values(next).map((t) => t.until);
    if (until.length) {
      this.expire = setTimeout(() => {
        const now = Date.now();
        this.setTypers(Object.fromEntries(Object.entries(this.typers).filter(([, t]) => t.until > now)));
      }, Math.max(0, Math.min(...until) - Date.now()) + 50);
    }
    publishTypers(this.id, next);
  }

  private async connect() {
    const token = await getAccessToken();
    if (this.stopped || !token) return;
    const ws = new WebSocket(`${wsBaseUrl}/ws/chat`);
    this.ws = ws;
    this.canType = false;
    // The token goes in the first message, never the URL (URLs end up in logs).
    ws.onopen = () => {
      const afterId = lastId(this.qc.getQueryData<RoomData>(keys.room(this.room)));
      ws.send(JSON.stringify({ token, room: this.room, afterId }));
    };
    ws.onmessage = (e) => {
      const frame = parseFrame(e.data);
      if (!frame) return;
      if (frame.type === 'ready') {
        this.canType = frame.typing;
        this.attempt = 0;
      } else if (frame.type === 'typing') {
        const { userId, name } = frame;
        this.setTypers({ ...this.typers, [userId]: { name, until: Date.now() + typingShown } });
      } else {
        this.qc.setQueryData<RoomData>(keys.room(this.room), (cur) => cur && { ...cur, messages: merge(cur.messages, frame.messages) });
        // Someone whose message just arrived has stopped typing.
        const sent = frame.messages.flatMap((m) => (m.userId && this.typers[m.userId] ? [m.userId] : []));
        if (sent.length) this.setTypers(Object.fromEntries(Object.entries(this.typers).filter(([who]) => !sent.includes(who))));
      }
    };
    ws.onclose = (e) => {
      if (this.ws === ws) this.ws = null;
      if (this.stopped) return;
      if (permanent.has(e.code)) {
        this.stopped = true;
        // Left or removed: ask the API, whose answer puts the room in its new state.
        void this.qc.invalidateQueries({ queryKey: keys.room(this.room) });
        return;
      }
      // 1 s, 2 s, 4 s … up to 30 s. A 4401 (expired token) gets a fresh one next time.
      this.reconnect = setTimeout(() => void this.connect(), Math.min(30_000, 1000 * 2 ** this.attempt++));
    };
  }

  /** Tells the others the person is typing, at most every couple of seconds and only once the server said it takes notices. */
  typing() {
    const now = Date.now();
    if (!this.canType || this.ws?.readyState !== WebSocket.OPEN || now - this.lastTyped < typingEvery) return;
    this.lastTyped = now;
    this.ws.send(JSON.stringify({ type: 'typing' }));
  }

  close() {
    this.stopped = true;
    clearTimeout(this.reconnect);
    clearTimeout(this.expire);
    this.ws?.close();
    this.ws = null;
    publishTypers(this.id, nobody);
  }
}

// One socket per room, however many screens show it.
const sockets = new Map<string, RoomSocket>();

/** Keeps the room's socket open until the returned function is called; the last release closes it. */
function retain(qc: QueryClient, room: RoomRef): () => void {
  const id = roomId(room);
  let socket = sockets.get(id);
  if (!socket) {
    socket = new RoomSocket(qc, room);
    sockets.set(id, socket);
  }
  const mine = socket;
  mine.refs++;
  return () => {
    if (--mine.refs > 0) return;
    mine.close();
    if (sockets.get(id) === mine) sockets.delete(id);
  };
}

/**
 * A chat room (an event group's or a crew's): the latest messages from the
 * API, new ones pushed over one WebSocket shared by every screen showing the
 * room, all kept in the query cache.
 *
 * `status` is the server's word: `open` when it said the person may write
 * now. The device clock only schedules asking again when the room next opens
 * or closes, and tells "not open yet" from "closed" when the server says no.
 *
 * `typing` is the names of others typing now; call `onTyping` as the person
 * types.
 */
export function useRoom(room: RoomRef) {
  const qc = useQueryClient();
  const id = roomId(room);
  const query = useQuery({
    queryKey: keys.room(room),
    queryFn: () => fetchRoom(qc, room),
    // A room opening or closing changes what the person may do, so ask again then.
    refetchInterval: (q) => untilNextBoundary(q.state.data, Date.now()) ?? false,
    staleTime: 0,
    retry: (count, err) => !denied(err) && count < 2,
  });

  // If the device clock runs ahead of the server's, asking at a boundary can
  // come back with the old state (not open yet past opensAt, still open past
  // closesAt). Ask again a few times with growing waits, then leave it to the
  // next mount or app focus.
  const retries = useRef(0);
  const { data: current, dataUpdatedAt, refetch } = query;
  useEffect(() => {
    if (!current) return;
    const now = Date.now();
    const contradicted = current.open
      ? current.closesAt !== undefined && current.closesAt <= now
      : current.opensAt !== undefined && current.opensAt <= now && (current.closesAt === undefined || now < current.closesAt);
    if (!contradicted) {
      retries.current = 0;
      return;
    }
    const delay = skewRetryDelays[retries.current];
    if (delay === undefined) return;
    const timer = setTimeout(() => {
      retries.current++;
      void refetch();
    }, delay);
    return () => clearTimeout(timer);
  }, [current, dataUpdatedAt, refetch]);

  const notMember = denied(query.error);
  // The socket needs a room the server let the person read.
  const readable = !!query.data && !notMember;
  useEffect(() => (readable ? retain(qc, roomFromId(id)) : undefined), [qc, readable, id]);

  const typers = useSyncExternalStore(
    useCallback((listener: () => void) => subscribeTyping(id, listener), [id]),
    () => typersByRoom.get(id) ?? nobody,
  );
  const typing = Object.values(typers).map((t) => t.name);
  const onTyping = useCallback(() => sockets.get(id)?.typing(), [id]);

  const send = useMutation({
    mutationFn: async (body: string) => (await api.chat.sendMessage({ ...roomOf(room), body })).message,
    // Shown at once; the socket brings the same message (same id) again.
    onSuccess: (message) => {
      if (message) qc.setQueryData<RoomData>(keys.room(room), (cur) => cur && { ...cur, messages: merge(cur.messages, [fromApi(message)]) });
    },
  });

  const data = query.data;
  let status: RoomStatus;
  if (notMember) status = 'notMember';
  else if (query.isPending) status = 'loading';
  else if (!data) status = 'error';
  else if (data.open) status = 'open';
  else status = data.notYet ? 'notOpen' : 'closed';

  return {
    messages: data?.messages ?? [],
    status,
    opensAt: data?.opensAt,
    closesAt: data?.closesAt,
    send,
    typing,
    onTyping,
    retry: () => void query.refetch(),
  };
}
