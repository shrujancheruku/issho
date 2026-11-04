// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { fireEvent, render, screen } from '@testing-library/react-native';

import PlansScreen from '@/app/(tabs)/plans';

import { useFeed } from '../data';

// The tab's own code and the real frame run; data, map and styling are faked.
jest.mock('../data', () => ({ useFeed: jest.fn() }));
jest.mock('../names', () => ({ usePlanNames: () => mockNames }));
jest.mock('../plan-card', () => ({ PlanCard: () => null }));
jest.mock('../plans-map', () => ({ PlansMap: () => null }));
jest.mock('../map-data', () => ({ pinsFor: () => [] }));
jest.mock('../sections', () => ({ planSections: () => [], planCategories: () => [] }));
jest.mock('../level', () => ({ levelColors: () => ({}) }));
jest.mock('@/components/map-sheet', () => ({ MapSheet: () => null }));
jest.mock('@/components/tab-icons', () => ({ TabIcon: () => null }));
jest.mock('@/components/pressable-scale', () => {
  const { Pressable } = jest.requireActual('react-native');
  return { PressableScale: Pressable };
});
jest.mock('@/components/chip', () => ({ Chip: () => null }));
jest.mock('@/components/language-switch', () => ({ LanguageSwitch: () => null }));
jest.mock('@/lib/haptics', () => ({ haptics: {} }));
jest.mock('react-native-svg', () => ({ __esModule: true, default: () => null, Circle: () => null, Path: () => null }));
jest.mock('expo-router', () => ({ router: { canGoBack: jest.fn(), back: jest.fn(), replace: jest.fn() } }));
jest.mock('@/i18n/i18n-provider', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
jest.mock('@/i18n/strings', () => ({ english: {} }));
jest.mock('@/constants/theme', () => ({
  BodyFont: { regular: 'x' },
  FontSize: new Proxy({}, { get: () => 16 }),
  BottomTabInset: 0,
  FormWidth: 600,
  MaxContentWidth: 800,
  Spacing: new Proxy({}, { get: () => 4 }),
}));
jest.mock('@/hooks/use-theme', () => ({ useTheme: () => new Proxy({}, { get: () => '#000' }) }));
jest.mock('@/components/themed-view', () => ({ ThemedView: ({ children }: { children: unknown }) => children }));
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: unknown }) => children,
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
}));
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

const mockNames = { ready: true, category: (c: string) => c };
const mockUseFeed = useFeed as unknown as jest.Mock;
const feed = (over: Record<string, unknown>) => ({
  isPending: false,
  isError: false,
  isSuccess: false,
  isRefetching: false,
  error: null,
  data: undefined,
  refetch: jest.fn(),
  ...over,
});
const spinning = () => JSON.stringify(screen.toJSON()).includes('"ActivityIndicator"');

describe('Plans tab', () => {
  it('keeps its title while loading, with a spinner', async () => {
    mockUseFeed.mockReturnValue(feed({ isPending: true }));
    await render(<PlansScreen />);
    expect(screen.getByText('plans.title')).toBeTruthy();
    expect(spinning()).toBe(true);
  });

  it('keeps its title when the feed fails, and retries', async () => {
    const failed = feed({ isError: true, error: new Error('down') });
    mockUseFeed.mockReturnValue(failed);
    await render(<PlansScreen />);
    expect(screen.getByText('plans.title')).toBeTruthy();
    expect(screen.getByText('common.loadFailed')).toBeTruthy();
    await fireEvent.press(screen.getByText('common.retry'));
    expect(failed.refetch).toHaveBeenCalledTimes(1);
  });

  it('shows the empty list once loaded', async () => {
    mockUseFeed.mockReturnValue(feed({ isSuccess: true, data: [] }));
    await render(<PlansScreen />);
    expect(screen.getByText('feed.empty')).toBeTruthy();
  });
});
