/// <reference types="jest" />
import { merge, parseFrame, suggestedPlans, type Message } from '../messages';

const m = (id: number, body = `m${id}`): Message => ({ id, name: 'A', body, at: new Date(0), mine: false });

it('merges messages by id, oldest first, without duplicates', () => {
  // A sent message comes back from the socket with the same id.
  expect(merge([m(1), m(3)], [m(2), m(3, 'again')]).map((x) => [x.id, x.body])).toEqual([
    [1, 'm1'],
    [2, 'm2'],
    [3, 'again'],
  ]);
  const same = [m(1)];
  expect(merge(same, [])).toBe(same);
});

it('lists the plans suggested in a chat once each, newest first', () => {
  const plan = (id: number, planId: string): Message => ({ ...m(id), planId });
  expect(suggestedPlans([plan(1, 'a'), m(2), plan(3, 'b'), plan(4, 'a')])).toEqual(['b', 'a']);
  expect(suggestedPlans([m(1)])).toEqual([]);
});

describe('parseFrame', () => {
  const wire = { id: 3, userId: 'u', name: 'A', body: 'hi', at: '2030-01-01T00:00:00Z', mine: true };

  it('reads what the server sends', () => {
    expect(parseFrame('{"type":"ready","typing":true}')).toEqual({ type: 'ready', typing: true });
    expect(parseFrame('{"type":"ready"}')).toEqual({ type: 'ready', typing: false });
    expect(parseFrame('{"type":"typing","userId":"u","name":"Aiko"}')).toEqual({ type: 'typing', userId: 'u', name: 'Aiko' });
    expect(parseFrame(JSON.stringify({ type: 'messages', messages: [{ ...wire, planId: 'p' }] }))).toEqual({
      type: 'messages',
      messages: [{ id: 3, userId: 'u', name: 'A', body: 'hi', at: new Date('2030-01-01T00:00:00Z'), mine: true, planId: 'p' }],
    });
  });

  it('drops what it cannot trust', () => {
    const raws = ['', 'nope', 'null', '[]', '1', '{}', '{"type":"x"}', '{"type":"typing","name":"A"}', '{"type":"typing","userId":"","name":"A"}', '{"type":"messages"}'];
    for (const raw of raws) expect(parseFrame(raw)).toBeNull();
  });

  it('drops malformed messages from a batch and keeps the rest', () => {
    const bad = [{ ...wire, id: '3' }, { ...wire, id: 0 }, { ...wire, id: 1.5 }, { ...wire, at: 'soon' }, { ...wire, mine: 'yes' }, { ...wire, body: null }, { ...wire, planId: 7 }, 'x', null];
    const frame = parseFrame(JSON.stringify({ type: 'messages', messages: [...bad, wire] }));
    expect(frame?.type === 'messages' && frame.messages.map((x) => x.id)).toEqual([3]);
  });
});
