// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { fireEvent, render, screen } from '@testing-library/react-native';

import SignupSheet from '@/app/signup/[id]';

import { usePlanTimes } from '../use-plan-times';

// The sheet's own code runs; the data hooks and the styling are faked.
jest.mock('../use-plan-times', () => ({ usePlanTimes: jest.fn() }));
jest.mock('../data', () => ({ useSignupActions: () => ({ signUp: { mutate: jest.fn() } }) }));
jest.mock('@/features/auth/auth-provider', () => ({ useAuth: () => ({ session: null }) }));
jest.mock('@/lib/push', () => ({ registerForPush: jest.fn() }));
jest.mock('@/lib/haptics', () => ({ haptics: {} }));
jest.mock('expo-router', () => ({
  router: { canGoBack: jest.fn(), back: jest.fn(), replace: jest.fn() },
  useLocalSearchParams: () => ({ id: 'plan-1' }),
}));
jest.mock('@/i18n/i18n-provider', () => ({ useI18n: () => ({ t: (key: string) => key, lang: 'en' }) }));
jest.mock('@/constants/theme', () => ({ FormWidth: 600, Spacing: new Proxy({}, { get: () => 4 }) }));
jest.mock('@/hooks/use-theme', () => ({ useTheme: () => new Proxy({}, { get: () => '#000' }) }));
jest.mock('@/components/themed-view', () => ({ ThemedView: ({ children }: { children: unknown }) => children }));
jest.mock('@/components/availability-grid', () => ({ AvailabilityGrid: () => null }));
jest.mock('@/components/chip', () => ({ Chip: () => null }));
jest.mock('@/components/primary-button', () => ({ PrimaryButton: () => null }));
jest.mock('@/features/plans/fact', () => ({ Heading: () => null }));
jest.mock('react-native-safe-area-context', () => ({ SafeAreaView: ({ children }: { children: unknown }) => children }));
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

const mockUsePlanTimes = usePlanTimes as unknown as jest.Mock;

const query = (over: Record<string, unknown>) => ({
  isSuccess: false,
  isError: false,
  error: null,
  data: undefined,
  refetch: jest.fn(),
  ...over,
});

describe('SignupSheet', () => {
  it('offers a retry when the profile fails to load, instead of spinning forever', async () => {
    const profile = query({ isError: true, error: new Error('down') });
    mockUsePlanTimes.mockReturnValue({
      plan: query({ isSuccess: true, data: { plan: { id: 'plan-1' } } }),
      profile,
      usual: [],
      saved: null,
    });
    await render(<SignupSheet />);

    expect(JSON.stringify(screen.toJSON())).not.toContain('"ActivityIndicator"');
    expect(screen.getByText('common.loadFailed')).toBeTruthy();
    await fireEvent.press(screen.getByText('common.retry'));
    expect(profile.refetch).toHaveBeenCalledTimes(1);
  });
});
