/// <reference types="jest" />
import { merge, type Message } from '../chat-messages';

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
