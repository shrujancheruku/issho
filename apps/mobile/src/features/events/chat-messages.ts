import type { ChatMessage, CrewMessage } from '@issho/api-client';

/**
 * A chat message in the app: from the API (ChatMessage, CrewMessage) or the
 * socket. `planId`: a plan Issho suggested in a crew's chat.
 */
export type Message = { id: number; name: string; body: string; at: Date; mine: boolean; planId?: string };

export function fromApi(m: ChatMessage | CrewMessage): Message {
  const planId = 'planId' in m && m.planId ? m.planId : undefined;
  return { id: Number(m.id), name: m.name, body: m.body, at: new Date(Number(m.at?.seconds ?? 0) * 1000), mine: m.mine, planId };
}

/** Adds messages, keeping one per id, oldest first. */
export function merge(current: Message[], incoming: Message[]): Message[] {
  if (!incoming.length) return current;
  const byId = new Map(current.map((m) => [m.id, m]));
  for (const m of incoming) byId.set(m.id, m);
  return [...byId.values()].sort((a, b) => a.id - b.id);
}
