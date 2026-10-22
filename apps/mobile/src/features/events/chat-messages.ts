import type { ChatMessage } from '@issho/api-client';

/** A chat message in the app: from the API (ChatMessage) or the socket. */
export type Message = { id: number; name: string; body: string; at: Date; mine: boolean };

export function fromApi(m: ChatMessage): Message {
  return { id: Number(m.id), name: m.name, body: m.body, at: new Date(Number(m.at?.seconds ?? 0) * 1000), mine: m.mine };
}

/** Adds messages, keeping one per id, oldest first. */
export function merge(current: Message[], incoming: Message[]): Message[] {
  if (!incoming.length) return current;
  const byId = new Map(current.map((m) => [m.id, m]));
  for (const m of incoming) byId.set(m.id, m);
  return [...byId.values()].sort((a, b) => a.id - b.id);
}
