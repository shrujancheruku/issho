import type { ChatMessage } from '@issho/api-client';

/**
 * A chat message in the app, from the API (ChatMessage) or the socket.
 * `userId`: the sender (none for a suggested plan); `planId`: a plan Issho
 * suggested in a crew's chat.
 */
export type Message = { id: number; userId?: string; name: string; body: string; at: Date; mine: boolean; planId?: string };

export function fromApi(m: ChatMessage): Message {
  return {
    id: Number(m.id),
    userId: m.userId || undefined,
    name: m.name,
    body: m.body,
    at: new Date(Number(m.at?.seconds ?? 0) * 1000),
    mine: m.mine,
    planId: m.planId || undefined,
  };
}

/** Adds messages, keeping one per id, oldest first. */
export function merge(current: Message[], incoming: Message[]): Message[] {
  if (!incoming.length) return current;
  const byId = new Map(current.map((m) => [m.id, m]));
  for (const m of incoming) byId.set(m.id, m);
  return [...byId.values()].sort((a, b) => a.id - b.id);
}

/** The plans suggested in a crew's chat, newest first, each once. */
export function suggestedPlans(messages: Message[]): string[] {
  return [...new Set(messages.flatMap((m) => (m.planId ? [m.planId] : [])))].reverse();
}

/** What the socket can say (backend internal/chatws). */
export type Frame =
  | { type: 'ready'; typing: boolean }
  | { type: 'messages'; messages: Message[] }
  | { type: 'typing'; userId: string; name: string };

const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const optionalString = (v: unknown) => v === undefined || typeof v === 'string';

function parseMessage(m: unknown): Message | null {
  if (!isRecord(m)) return null;
  const { id, userId, name, body, at, mine, planId } = m;
  if (typeof id !== 'number' || !Number.isSafeInteger(id) || id <= 0) return null;
  if (typeof name !== 'string' || typeof body !== 'string' || typeof mine !== 'boolean' || typeof at !== 'string') return null;
  if (!optionalString(userId) || !optionalString(planId)) return null;
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return null;
  return { id, userId: userId || undefined, name, body, at: date, mine, planId: planId || undefined };
}

/**
 * Reads one socket frame. Anything that isn't what the server sends is
 * dropped (null), and so is a malformed message inside a batch, so a bad
 * payload can't put a broken row in the chat.
 */
export function parseFrame(raw: unknown): Frame | null {
  let data: unknown;
  try {
    data = JSON.parse(String(raw));
  } catch {
    return null;
  }
  if (!isRecord(data)) return null;
  switch (data.type) {
    case 'ready':
      return { type: 'ready', typing: data.typing === true };
    case 'typing':
      return typeof data.userId === 'string' && data.userId && typeof data.name === 'string' ? { type: 'typing', userId: data.userId, name: data.name } : null;
    case 'messages':
      return Array.isArray(data.messages) ? { type: 'messages', messages: data.messages.flatMap((m) => parseMessage(m) ?? []) } : null;
  }
  return null;
}
