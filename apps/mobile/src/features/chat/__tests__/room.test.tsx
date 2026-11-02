// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { Code, ConnectError } from '@issho/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react-native';
import type { ReactNode } from 'react';

import { api } from '@/lib/api';

import { useRoom } from '../room';

// The real hook, cache and socket code run; only the edges are faked: the
// API client, the access token and WebSocket.
jest.mock('@/lib/api', () => ({
  api: { chat: { listMessages: jest.fn(), sendMessage: jest.fn() } },
  getAccessToken: jest.fn(async () => 'token'),
  wsBaseUrl: 'ws://test',
}));

const mockApi = api as unknown as { chat: { listMessages: jest.Mock; sendMessage: jest.Mock } };

/** A WebSocket the test drives by hand. */
class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: FakeWebSocket[] = [];
  readyState = FakeWebSocket.CONNECTING;
  sent: Record<string, unknown>[] = [];
  closedByApp = false;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: ((e: { code: number }) => void) | null = null;

  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }

  send(data: string) {
    this.sent.push(JSON.parse(data));
  }

  close() {
    this.closedByApp = true;
    this.readyState = FakeWebSocket.CLOSED;
  }

  // What the server does.
  async open() {
    this.readyState = FakeWebSocket.OPEN;
    await act(async () => {
      this.onopen?.();
      jest.advanceTimersByTime(0);
    });
  }

  async receive(frame: unknown) {
    await act(async () => {
      this.onmessage?.({ data: typeof frame === 'string' ? frame : JSON.stringify(frame) });
      jest.advanceTimersByTime(0);
    });
  }

  async dropped(code = 1006) {
    this.readyState = FakeWebSocket.CLOSED;
    await act(async () => {
      this.onclose?.({ code });
      jest.advanceTimersByTime(0);
    });
  }
}

const sockets = () => FakeWebSocket.instances;
const apiMessage = (id: number, userId = 'u1', body = `m${id}`) => ({ id: BigInt(id), userId, name: 'Aiko', body, at: { seconds: 1000n }, mine: false, planId: '' });
const wireMessage = (id: number, userId = 'u1') => ({ id, userId, name: 'Aiko', body: `m${id}`, at: '2030-01-01T00:00:00Z', mine: false });
const response = (over: Record<string, unknown> = {}) => ({ messages: [], open: true, opensAt: undefined, closesAt: undefined, ...over });
const seconds = (ms: number) => ({ seconds: BigInt(Math.floor(ms / 1000)) });

let client: QueryClient;
const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;

/**
 * Lets pending promises (the API call, the token) settle. React Query hands
 * results to React from a zero-delay timer, which fake timers hold back.
 */
const flush = async () => {
  for (let i = 0; i < 6; i++) await act(async () => jest.advanceTimersByTime(0));
};

async function mount(room: Parameters<typeof useRoom>[0] = { eventId: 'e1' }) {
  const view = await renderHook(() => useRoom(room), { wrapper });
  await flush();
  return view;
}

beforeEach(() => {
  jest.useFakeTimers({ now: new Date('2030-01-01T12:00:00Z') });
  FakeWebSocket.instances = [];
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeWebSocket;
  client = new QueryClient({ defaultOptions: { queries: { gcTime: Infinity } } });
  mockApi.chat.listMessages.mockReset().mockResolvedValue(response({ messages: [apiMessage(5)] }));
  mockApi.chat.sendMessage.mockReset();
});

afterEach(() => {
  client.clear();
  jest.useRealTimers();
});

it('shares one socket between everyone showing a room, until the last one leaves', async () => {
  // Both are on screen together: the crew screen and its chat tab.
  const a = await renderHook(() => useRoom({ crewId: 'c1' }), { wrapper });
  const b = await renderHook(() => useRoom({ crewId: 'c1' }), { wrapper });
  await flush();
  expect(sockets()).toHaveLength(1);
  expect(a.result.current.messages.map((m) => m.id)).toEqual([5]);

  // Both see what the socket brings.
  await sockets()[0].open();
  await sockets()[0].receive({ type: 'messages', messages: [wireMessage(6)] });
  expect(a.result.current.messages.map((m) => m.id)).toEqual([5, 6]);
  expect(b.result.current.messages.map((m) => m.id)).toEqual([5, 6]);

  await a.unmount();
  expect(sockets()[0].closedByApp).toBe(false);
  await b.unmount();
  expect(sockets()[0].closedByApp).toBe(true);

  // Another room has its own.
  await mount({ crewId: 'c2' });
  expect(sockets()).toHaveLength(2);
});

it('sends the token, room and newest message id as the hello', async () => {
  await mount({ crewId: 'c1' });
  await sockets()[0].open();
  expect(sockets()[0].url).toBe('ws://test/ws/chat');
  expect(sockets()[0].sent).toEqual([{ token: 'token', room: { crewId: 'c1' }, afterId: 5 }]);
});

it('reconnects after a drop, asking for everything after the last message it has', async () => {
  const { result } = await mount();
  await sockets()[0].open();
  await sockets()[0].receive({ type: 'ready', typing: true });
  await sockets()[0].receive({ type: 'messages', messages: [wireMessage(9)] });
  await sockets()[0].dropped();

  await act(async () => jest.advanceTimersByTime(1000));
  await flush();
  expect(sockets()).toHaveLength(2);
  await sockets()[1].open();
  expect(sockets()[1].sent[0]).toMatchObject({ afterId: 9 });
  await sockets()[1].receive({ type: 'messages', messages: [wireMessage(10)] });
  expect(result.current.messages.map((m) => m.id)).toEqual([5, 9, 10]);
});

it('sends typing only after the server says ready, and not more than every 2 seconds', async () => {
  const { result } = await mount();
  const ws = sockets()[0];
  await ws.open();
  await act(async () => result.current.onTyping());
  expect(ws.sent).toHaveLength(1); // the hello only

  await ws.receive({ type: 'ready', typing: true });
  await act(async () => result.current.onTyping());
  expect(ws.sent[1]).toEqual({ type: 'typing' });
  await act(async () => result.current.onTyping());
  expect(ws.sent).toHaveLength(2);
  await act(async () => jest.advanceTimersByTime(2000));
  await act(async () => result.current.onTyping());
  expect(ws.sent).toHaveLength(3);
});

it('does not send typing to a server that does not take it', async () => {
  const { result } = await mount();
  const ws = sockets()[0];
  await ws.open();
  await ws.receive({ type: 'ready', typing: false });
  await act(async () => result.current.onTyping());
  expect(ws.sent).toHaveLength(1);
});

it('tells typers apart by person, not name, and clears one when their message arrives', async () => {
  const { result } = await mount();
  const ws = sockets()[0];
  await ws.open();
  await ws.receive({ type: 'typing', userId: 'u1', name: 'Aiko' });
  await ws.receive({ type: 'typing', userId: 'u2', name: 'Aiko' });
  expect(result.current.typing).toEqual(['Aiko', 'Aiko']);

  await ws.receive({ type: 'messages', messages: [wireMessage(6, 'u1')] });
  expect(result.current.typing).toEqual(['Aiko']);

  // Nobody keeps typing for ever.
  await act(async () => jest.advanceTimersByTime(6000));
  expect(result.current.typing).toEqual([]);
});

it('stops for good on a 4403 and asks the API where the person stands', async () => {
  const { result } = await mount();
  expect(result.current.status).toBe('open');
  mockApi.chat.listMessages.mockRejectedValue(new ConnectError('no', Code.PermissionDenied));
  await sockets()[0].open();
  await sockets()[0].dropped(4403);
  await flush();
  await act(async () => jest.advanceTimersByTime(60_000));
  await flush();

  expect(sockets()).toHaveLength(1);
  expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(2);
  expect(result.current.status).toBe('notMember');
});

it('is not a member when the API says permission denied, and opens no socket', async () => {
  mockApi.chat.listMessages.mockReset().mockRejectedValue(new ConnectError('no', Code.PermissionDenied));
  const { result } = await mount();
  expect(result.current.status).toBe('notMember');
  expect(sockets()).toHaveLength(0);
  expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(1); // no retries for a no
});

it('has an error, and retries, when history fails for another reason', async () => {
  mockApi.chat.listMessages.mockReset().mockRejectedValue(new ConnectError('down', Code.Unavailable));
  const { result } = await mount();
  // Two retries, a second and two apart.
  for (let i = 0; i < 4; i++) {
    await act(async () => jest.advanceTimersByTime(2000));
    await flush();
  }
  expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(3);
  expect(result.current.status).toBe('error');
  expect(sockets()).toHaveLength(0);

  mockApi.chat.listMessages.mockResolvedValue(response());
  await act(async () => result.current.retry());
  await flush();
  expect(result.current.status).toBe('open');
  expect(sockets()).toHaveLength(1);
});

describe('status follows the server', () => {
  const now = new Date('2030-01-01T12:00:00Z').getTime();

  it('is open when the server says so, even without times', async () => {
    mockApi.chat.listMessages.mockResolvedValue(response({ open: true }));
    const { result } = await mount();
    expect(result.current.status).toBe('open');
    expect(result.current.opensAt).toBeUndefined();
  });

  it('is open whatever the device clock says about the window', async () => {
    // The window ended long ago by the device's clock; the server says open.
    mockApi.chat.listMessages.mockResolvedValue(response({ open: true, opensAt: seconds(now - 9e8), closesAt: seconds(now - 8e8) }));
    expect((await mount()).result.current.status).toBe('open');
  });

  it('is not open yet before the opening, and closed after the closing or with no schedule', async () => {
    mockApi.chat.listMessages.mockResolvedValue(response({ open: false, opensAt: seconds(now + 3600_000), closesAt: seconds(now + 7200_000) }));
    const early = await mount({ eventId: 'early' });
    expect(early.result.current.status).toBe('notOpen');
    expect(early.result.current.opensAt).toBe(now + 3600_000);

    mockApi.chat.listMessages.mockResolvedValue(response({ open: false, opensAt: seconds(now - 7200_000), closesAt: seconds(now - 3600_000) }));
    expect((await mount({ eventId: 'late' })).result.current.status).toBe('closed');

    mockApi.chat.listMessages.mockResolvedValue(response({ open: false }));
    expect((await mount({ eventId: 'none' })).result.current.status).toBe('closed');
  });

  it('asks again when the room next opens, and shows what the server then says', async () => {
    mockApi.chat.listMessages.mockResolvedValue(response({ open: false, opensAt: seconds(now + 600_000), closesAt: seconds(now + 6000_000) }));
    const { result } = await mount();
    expect(result.current.status).toBe('notOpen');
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(1);

    mockApi.chat.listMessages.mockResolvedValue(response({ open: true, opensAt: seconds(now + 600_000), closesAt: seconds(now + 6000_000) }));
    await act(async () => jest.advanceTimersByTime(599_000));
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(1);
    await act(async () => jest.advanceTimersByTime(2_000));
    await flush();
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(2);
    expect(result.current.status).toBe('open');
  });

  it('asks again after a short wait when the boundary answer is behind (device clock 20 s ahead)', async () => {
    const times = { opensAt: seconds(now + 600_000), closesAt: seconds(now + 6000_000) };
    mockApi.chat.listMessages.mockResolvedValue(response({ open: false, ...times }));
    const { result } = await mount();

    // The device reaches the opening 20 s before the server does, so the
    // answer at the boundary still says not open; the next one says open.
    mockApi.chat.listMessages
      .mockResolvedValueOnce(response({ open: false, ...times }))
      .mockResolvedValueOnce(response({ open: true, ...times }));
    await act(async () => jest.advanceTimersByTime(602_000));
    await flush();
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(2);
    expect(result.current.status).toBe('notOpen');

    await act(async () => jest.advanceTimersByTime(14_000));
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(2);
    await act(async () => jest.advanceTimersByTime(2_000));
    await flush();
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(3);
    expect(result.current.status).toBe('open');
  });

  it('gives up after three waits of 15, 30 and 60 seconds', async () => {
    const times = { opensAt: seconds(now + 60_000) };
    mockApi.chat.listMessages.mockResolvedValue(response({ open: false, ...times }));
    await mount();
    for (const wait of [62_000, 15_000, 30_000, 60_000, 300_000]) {
      await act(async () => jest.advanceTimersByTime(wait));
      await flush();
    }
    expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(5);
  });

  it('asks again when the room closes', async () => {
    mockApi.chat.listMessages.mockResolvedValue(response({ open: true, opensAt: seconds(now - 600_000), closesAt: seconds(now + 300_000) }));
    const { result } = await mount();
    expect(result.current.status).toBe('open');

    mockApi.chat.listMessages.mockResolvedValue(response({ open: false, opensAt: seconds(now - 600_000), closesAt: seconds(now + 300_000) }));
    await act(async () => jest.advanceTimersByTime(302_000));
    await flush();
    expect(result.current.status).toBe('closed');
  });

  it('asks only for messages after the newest it has when it asks again', async () => {
    mockApi.chat.listMessages.mockResolvedValue(response({ messages: [apiMessage(5)], open: false, opensAt: seconds(now + 60_000) }));
    const { result } = await mount();
    await sockets()[0].open();
    await sockets()[0].receive({ type: 'messages', messages: [wireMessage(8)] });

    mockApi.chat.listMessages.mockResolvedValue(response({ messages: [apiMessage(9)], open: true }));
    await act(async () => jest.advanceTimersByTime(62_000));
    await flush();
    expect(mockApi.chat.listMessages).toHaveBeenLastCalledWith(expect.objectContaining({ afterId: 8n }));
    expect(result.current.messages.map((m) => m.id)).toEqual([5, 8, 9]);
  });
});

it('drops malformed socket payloads', async () => {
  const { result } = await mount();
  const ws = sockets()[0];
  await ws.open();
  await ws.receive('not json');
  await ws.receive('null');
  await ws.receive({ type: 'messages' });
  await ws.receive({ type: 'messages', messages: 'nope' });
  await ws.receive({ type: 'surprise' });
  await ws.receive({ type: 'typing', name: 'no id' });
  await ws.receive({
    type: 'messages',
    messages: [
      { id: 'x', name: 'A', body: 'b', at: '2030-01-01T00:00:00Z', mine: false },
      { id: 6, name: 'A', body: 'no time', mine: false },
      { id: 7, name: 'A', body: 'bad time', at: 'yesterday-ish', mine: false },
      { id: 8, name: 'A', body: 42, at: '2030-01-01T00:00:00Z', mine: false },
      null,
      wireMessage(10),
    ],
  });
  expect(result.current.messages.map((m) => m.id)).toEqual([5, 10]);
  expect(result.current.typing).toEqual([]);
});

it('shows a sent message at once, and once when the socket brings it again', async () => {
  const { result } = await mount();
  mockApi.chat.sendMessage.mockResolvedValue({ message: apiMessage(6, 'me', 'hello') });
  await act(async () => result.current.send.mutate('hello'));
  await flush();
  expect(mockApi.chat.sendMessage).toHaveBeenCalledWith({ room: { room: { case: 'eventId', value: 'e1' } }, body: 'hello' });
  expect(result.current.messages.map((m) => m.id)).toEqual([5, 6]);

  await sockets()[0].open();
  await sockets()[0].receive({ type: 'messages', messages: [wireMessage(6, 'me')] });
  expect(result.current.messages.map((m) => m.id)).toEqual([5, 6]);
});
