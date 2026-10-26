import { useMutation, useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState } from 'react';

import { api, getAccessToken, wsBaseUrl } from '@/lib/api';

import { fromApi, merge, type Message } from './chat-messages';

// Close codes from the server (backend internal/chatws) after which
// reconnecting can't help.
const permanent = new Set([4403, 4404]);

// Typing: tell the others at most this often while the person types, and
// show someone as typing for this long after their last notice (their
// app re-sends every couple of seconds while they keep typing).
const typingEvery = 2000;
const typingShown = 5000;

/** Whose chat: an event group's (the caller's group) or a crew's. */
export type ChatTarget = { eventId: string } | { crewId: string };

/**
 * A group chat (an event group's or a crew's): the latest messages from the
 * API, then new ones pushed over a WebSocket (GET /ws/chat). The socket
 * reconnects with backoff and asks for everything after the last message
 * it has, so nothing is missed while it was down.
 *
 * The socket also carries typing: `typing` is who else is typing now, and
 * `onTyping` is called as the person types. It only sends once the server
 * has said it takes typing notices ("ready"), as older servers close a
 * socket that sends them.
 */
export function useChat(target: ChatTarget) {
  const eventId = 'eventId' in target ? target.eventId : '';
  const crewId = 'crewId' in target ? target.crewId : '';
  const [messages, setMessages] = useState<Message[]>([]);
  const lastId = useRef(0);
  const add = useCallback((ms: Message[]) => {
    setMessages((cur) => {
      const next = merge(cur, ms);
      lastId.current = next.length ? next[next.length - 1].id : lastId.current;
      return next;
    });
  }, []);

  const history = useQuery({
    queryKey: ['chat', eventId, crewId],
    queryFn: async () =>
      eventId ? api.events.listChatMessages({ eventId }) : { ...(await api.crews.listCrewMessages({ crewId })), opensAt: undefined, closesAt: undefined, open: true },
    enabled: !!(eventId || crewId),
  });
  useEffect(() => {
    if (history.data) add(history.data.messages.map(fromApi));
  }, [history.data, add]);

  const [live, setLive] = useState(false);
  // Who's typing, until when (ms).
  const [typers, setTypers] = useState<Record<string, number>>({});
  // Sends a typing notice when the socket can; replaced by each connection.
  const sendTyping = useRef<() => void>(() => {});
  const readable = !!history.data; // the API said we may read it
  useEffect(() => {
    if (!readable) return;
    let socket: WebSocket | null = null;
    let stopped = false;
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const connect = async () => {
      const token = await getAccessToken();
      if (stopped || !token) return;
      const ws = new WebSocket(`${wsBaseUrl}/ws/chat`);
      socket = ws;
      let canType = false;
      let lastTyped = 0;
      sendTyping.current = () => {
        const now = Date.now();
        if (!canType || ws.readyState !== WebSocket.OPEN || now - lastTyped < typingEvery) return;
        lastTyped = now;
        ws.send(JSON.stringify({ type: 'typing' }));
      };
      // The token goes in the first message, never the URL (URLs end up in logs).
      socket.onopen = () => {
        attempt = 0;
        setLive(true);
        socket?.send(JSON.stringify({ token, ...(eventId ? { eventId } : { crewId }), afterId: lastId.current }));
      };
      socket.onmessage = (e) => {
        try {
          const data = JSON.parse(String(e.data)) as {
            type: string;
            messages?: (Omit<Message, 'at'> & { at: string })[];
            typing?: boolean;
            name?: string;
          };
          if (data.type === 'ready') canType = !!data.typing;
          if (data.type === 'typing' && data.name) {
            const name = data.name;
            setTypers((cur) => ({ ...cur, [name]: Date.now() + typingShown }));
          }
          if (data.type === 'messages' && data.messages) {
            const ms = data.messages.map((m) => ({ ...m, at: new Date(m.at) }));
            add(ms);
            // Someone whose message just arrived has stopped typing.
            setTypers((cur) => {
              const next = { ...cur };
              for (const m of ms) delete next[m.name];
              return next;
            });
          }
        } catch {
          // Not ours; ignore.
        }
      };
      socket.onclose = (e) => {
        setLive(false);
        if (stopped || permanent.has(e.code)) return;
        // 1 s, 2 s, 4 s … up to 30 s. A 4401 (expired token) gets a fresh one next time.
        timer = setTimeout(() => void connect(), Math.min(30_000, 1000 * 2 ** attempt++));
      };
    };
    void connect();
    return () => {
      stopped = true;
      clearTimeout(timer);
      socket?.close();
    };
  }, [readable, eventId, crewId, add]);

  // Drop people from the typing list as their time runs out.
  useEffect(() => {
    const until = Object.values(typers);
    if (!until.length) return;
    const timer = setTimeout(
      () => {
        const now = Date.now();
        setTypers((cur) => Object.fromEntries(Object.entries(cur).filter(([, u]) => u > now)));
      },
      Math.max(0, Math.min(...until) - Date.now()) + 50,
    );
    return () => clearTimeout(timer);
  }, [typers]);
  const typing = Object.keys(typers);
  const onTyping = useCallback(() => sendTyping.current(), []);

  const send = useMutation({
    mutationFn: async (body: string) =>
      (eventId ? await api.events.sendChatMessage({ eventId, body }) : await api.crews.sendCrewMessage({ crewId, body })).message,
    // Shown at once; the socket brings the same message (same id) again.
    onSuccess: (message) => message && add([fromApi(message)]),
  });

  return { messages, history, live, send, typing, onTyping };
}
