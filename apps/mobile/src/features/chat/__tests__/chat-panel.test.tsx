// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { Code, ConnectError } from '@issho/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';

import { api } from '@/lib/api';
import { keys } from '@/lib/queries';

import { ChatPanel } from '../chat-panel';
import { useRoom } from '../room';

// The real panel, hook and socket code run; only the edges are faked: the
// API client, the translations, navigation and WebSocket.
jest.mock('@/lib/api', () => ({
  api: { chat: { listMessages: jest.fn(), sendMessage: jest.fn() }, plans: { getPlan: jest.fn(), getPlanCards: jest.fn() } },
  getAccessToken: jest.fn(async () => 'token'),
  wsBaseUrl: 'ws://test',
}));
jest.mock('@/i18n/i18n-provider', () => ({
  useI18n: () => ({ t: (key: string) => key, lang: 'en', pick: (a: unknown) => ({ name: a }) }),
}));
jest.mock('expo-router', () => ({ router: { push: jest.fn() } }));
// Reanimated needs its native module; the panel only uses it to animate.
jest.mock('react-native-reanimated', () => {
  const { View } = jest.requireActual('react-native');
  return { __esModule: true, default: { View }, FadeInDown: { duration: () => ({ easing: () => ({}) }) } };
});
jest.mock('@/constants/motion', () => ({ EaseOut: {}, Motion: { move: 0 } }));
jest.mock('@/components/pressable-scale', () => ({ PressableScale: ({ children }: { children: unknown }) => children }));
jest.mock('../typing-dots', () => ({ TypingDots: () => null }));
// Styling and plain components are out of scope here (the theme module loads CSS).
jest.mock('@/constants/theme', () => ({
  BodyFont: { regular: 'body' },
  FontSize: new Proxy({}, { get: () => 16 }),
  Brand: { gold: 'gold', ink: 'ink' },
  Spacing: new Proxy({}, { get: () => 4 }),
}));
jest.mock('@/hooks/use-theme', () => ({ useTheme: () => new Proxy({}, { get: () => '#000' }) }));
jest.mock('@/components/themed-text', () => {
  const { Text } = jest.requireActual('react-native');
  return { ThemedText: ({ children }: { children: unknown }) => <Text>{children}</Text> };
});
jest.mock('@/components/text-link', () => {
  const { Pressable, Text } = jest.requireActual('react-native');
  return {
    TextLink: ({ label, onPress }: { label: string; onPress: () => void }) => (
      <Pressable onPress={onPress}>
        <Text>{label}</Text>
      </Pressable>
    ),
  };
});
jest.mock('@/features/plans/text', () => ({ pairText: () => '' }));
jest.mock('@/features/plans/names', () => ({ usePlanNames: () => ({ area: () => ({ name: '' }) }) }));

const mockApi = api as unknown as {
  chat: { listMessages: jest.Mock };
  plans: { getPlan: jest.Mock; getPlanCards: jest.Mock };
};

const sockets: object[] = [];
class FakeWebSocket {
  static OPEN = 1;
  readyState = 0;
  onopen = null;
  onmessage = null;
  onclose = null;
  constructor() {
    sockets.push(this);
  }
  send() {}
  close() {}
}

const reply = (over: Record<string, unknown> = {}) => ({
  messages: [{ id: 1n, userId: 'u', name: 'Aiko', body: 'hello', at: { seconds: 1000n }, mine: false, planId: '' }],
  open: true,
  ...over,
});

let client: QueryClient;
function renderPanel(ui: React.ReactElement) {
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

beforeEach(() => {
  client = new QueryClient();
  sockets.length = 0;
  (globalThis as unknown as { WebSocket: unknown }).WebSocket = FakeWebSocket;
  mockApi.chat.listMessages.mockReset().mockResolvedValue(reply());
  mockApi.plans.getPlan.mockReset();
  mockApi.plans.getPlanCards.mockReset().mockResolvedValue({ plans: [] });
});

// Open timers (garbage collection, retries) would keep Jest from exiting.
afterEach(() => client.clear());

it('shows the composer when the server says the room is open, with no opening time', async () => {
  await renderPanel(<ChatPanel room={{ crewId: 'c1' }} />);
  expect(await screen.findByText('hello')).toBeTruthy();
  expect(screen.getByLabelText('chat.toCrew')).toBeTruthy();
  expect(screen.queryByText('chat.closed')).toBeNull();
});

it('is not shown as closed for an event with no opening time that the server says is open', async () => {
  await renderPanel(<ChatPanel room={{ eventId: 'e1' }} />);
  expect(await screen.findByLabelText('chat.toGroup')).toBeTruthy();
  expect(screen.queryByText('chat.closed')).toBeNull();
});

it('says it is closed, without a composer, when the server says it is not open', async () => {
  mockApi.chat.listMessages.mockResolvedValue(reply({ open: false }));
  await renderPanel(<ChatPanel room={{ eventId: 'e1' }} />);
  expect(await screen.findByText('chat.closed')).toBeTruthy();
  expect(screen.queryByLabelText('chat.toGroup')).toBeNull();
});

it('says who can use it to someone who is not in the room', async () => {
  mockApi.chat.listMessages.mockRejectedValue(new ConnectError('no', Code.PermissionDenied));
  await renderPanel(<ChatPanel room={{ eventId: 'e1' }} />);
  expect(await screen.findByText('chat.notGoing')).toBeTruthy();
});

it('offers a retry, not "no messages yet", when history fails', async () => {
  mockApi.chat.listMessages.mockRejectedValue(new ConnectError('down', Code.Unavailable));
  await renderPanel(<ChatPanel room={{ eventId: 'e1' }} />);
  // The room asks again twice (a second, then two) before giving up.
  expect(await screen.findByText('common.loadFailed', {}, { timeout: 5000 })).toBeTruthy();
  expect(screen.queryByText('chat.empty')).toBeNull();

  mockApi.chat.listMessages.mockResolvedValue(reply({ messages: [] }));
  await fireEvent.press(screen.getByText('common.retry'));
  expect(await screen.findByText('chat.empty')).toBeTruthy();
});

it('keeps the messages and says so when a later refetch fails', async () => {
  // No waiting between the room's retries.
  client = new QueryClient({ defaultOptions: { queries: { retryDelay: 0 } } });
  await renderPanel(<ChatPanel room={{ eventId: 'e1' }} />);
  expect(await screen.findByText('hello')).toBeTruthy();

  mockApi.chat.listMessages.mockRejectedValue(new ConnectError('down', Code.Unavailable));
  void client.refetchQueries();
  // The room asks again twice before giving up.
  expect(await screen.findByText('common.refreshFailed')).toBeTruthy();
  expect(screen.getByText('hello')).toBeTruthy();
  expect(screen.queryByText('common.loadFailed')).toBeNull();
});

it('opens one socket for the crew chat tab, though the screen and the panel both use the room', async () => {
  // As app/crew/[id] does: the screen reads the room for its suggested plans
  // and the chat tab shows the panel.
  function CrewScreen() {
    useRoom({ crewId: 'c1' });
    return <ChatPanel room={{ crewId: 'c1' }} />;
  }
  await renderPanel(<CrewScreen />);
  expect(await screen.findByText('hello')).toBeTruthy();
  expect(sockets).toHaveLength(1);
  expect(mockApi.chat.listMessages).toHaveBeenCalledTimes(1);
});

const suggestion = (id: bigint, planId: string) => ({ id, userId: '', name: '', body: '', at: { seconds: 1000n }, mine: false, planId });
const card = (id: string) => ({ id, title: `Title ${id}`, secondaryTitle: '', areaId: 'a', signupDeadline: undefined, signedUp: false });

it('keeps the cards on screen when a new suggestion arrives and the batch is asked again', async () => {
  mockApi.chat.listMessages.mockResolvedValue(reply({ messages: [suggestion(1n, 'p1'), suggestion(2n, 'p2')] }));
  mockApi.plans.getPlanCards.mockResolvedValue({ plans: [card('p1'), card('p2')] });
  await renderPanel(<ChatPanel room={{ crewId: 'c1' }} />);
  expect(await screen.findByText('Title p2', {}, { timeout: 3000 })).toBeTruthy();

  // The next call never answers, so what shows is the placeholder.
  mockApi.plans.getPlanCards.mockReturnValue(new Promise(() => {}));
  mockApi.chat.listMessages.mockResolvedValue(reply({ messages: [suggestion(1n, 'p1'), suggestion(2n, 'p2'), suggestion(3n, 'p3')] }));
  await client.refetchQueries({ queryKey: keys.room({ crewId: 'c1' }) });
  await waitFor(() => expect(mockApi.plans.getPlanCards).toHaveBeenLastCalledWith({ planIds: ['p1', 'p2', 'p3'] }));
  expect(screen.getByText('Title p1')).toBeTruthy();
  expect(screen.getByText('Title p2')).toBeTruthy();
});

it('loads every suggested plan\'s card with one GetPlanCards call and no GetPlan', async () => {
  mockApi.chat.listMessages.mockResolvedValue(reply({ messages: [suggestion(1n, 'p1'), suggestion(2n, 'p2'), suggestion(3n, 'p3')] }));
  mockApi.plans.getPlanCards.mockResolvedValue({ plans: ['p1', 'p2', 'p3'].map(card) });
  await renderPanel(<ChatPanel room={{ crewId: 'c1' }} />);
  expect(await screen.findByText('Title p3', {}, { timeout: 3000 })).toBeTruthy();
  expect(screen.getByText('Title p1')).toBeTruthy();
  expect(mockApi.plans.getPlanCards).toHaveBeenCalledTimes(1);
  expect(mockApi.plans.getPlanCards).toHaveBeenCalledWith({ planIds: ['p1', 'p2', 'p3'] });
  expect(mockApi.plans.getPlan).not.toHaveBeenCalled();
});
