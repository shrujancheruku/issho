import { useMutation, useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState } from 'react';

import { api, getAccessToken, wsBaseUrl } from '@/lib/api';

import { fromApi, merge, type Message } from './chat-messages';

// Close codes from the server (backend internal/chatws) after which
// reconnecting can't help.
const permanent = new Set([4403, 4404]);

/**
 * An event's group chat: the latest messages from the API, then new ones
 * pushed over a WebSocket (GET /ws/chat). The socket reconnects with
 * backoff and asks for everything after the last message it has, so
 * nothing is missed while it was down.
 */
export function useChat(eventId: string) {
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
    queryKey: ['chat', eventId],
    queryFn: () => api.events.listChatMessages({ eventId }),
    enabled: !!eventId,
  });
  useEffect(() => {
    if (history.data) add(history.data.messages.map(fromApi));
  }, [history.data, add]);

  const [live, setLive] = useState(false);
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
      socket = new WebSocket(`${wsBaseUrl}/ws/chat`);
      // The token goes in the first message, never the URL (URLs end up in logs).
      socket.onopen = () => {
        attempt = 0;
        setLive(true);
        socket?.send(JSON.stringify({ token, eventId, afterId: lastId.current }));
      };
      socket.onmessage = (e) => {
        try {
          const data = JSON.parse(String(e.data)) as { type: string; messages: (Omit<Message, 'at'> & { at: string })[] };
          if (data.type === 'messages') add(data.messages.map((m) => ({ ...m, at: new Date(m.at) })));
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
  }, [readable, eventId, add]);

  const send = useMutation({
    mutationFn: (body: string) => api.events.sendChatMessage({ eventId, body }),
    // Shown at once; the socket brings the same message (same id) again.
    onSuccess: (res) => res.message && add([fromApi(res.message)]),
  });

  return { messages, history, live, send };
}
