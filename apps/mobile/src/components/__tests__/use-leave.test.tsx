// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { renderHook } from '@testing-library/react-native';
import { router } from 'expo-router';

import { useLeave } from '../screen-frame';

jest.mock('expo-router', () => ({ router: { canGoBack: jest.fn(), back: jest.fn(), replace: jest.fn() } }));
jest.mock('@/i18n/i18n-provider', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
jest.mock('@/constants/theme', () => ({ FormWidth: 600, Spacing: new Proxy({}, { get: () => 4 }) }));
jest.mock('@/components/themed-view', () => ({ ThemedView: ({ children }: { children: unknown }) => children }));
jest.mock('@/components/language-switch', () => ({ LanguageSwitch: () => null }));
jest.mock('@/components/text-link', () => ({ BackLink: () => null }));
jest.mock('react-native-safe-area-context', () => ({ SafeAreaView: ({ children }: { children: unknown }) => children }));

const mockRouter = router as unknown as { canGoBack: jest.Mock; back: jest.Mock; replace: jest.Mock };

describe('useLeave', () => {
  beforeEach(() => jest.clearAllMocks());

  it('goes back when there is a screen to go back to', async () => {
    mockRouter.canGoBack.mockReturnValue(true);
    (await renderHook(() => useLeave('/'))).result.current();
    expect(mockRouter.back).toHaveBeenCalledTimes(1);
    expect(mockRouter.replace).not.toHaveBeenCalled();
  });

  it('replaces the screen with the fallback when there is not', async () => {
    mockRouter.canGoBack.mockReturnValue(false);
    (await renderHook(() => useLeave('/profile'))).result.current();
    expect(mockRouter.replace).toHaveBeenCalledWith('/profile');
    expect(mockRouter.back).not.toHaveBeenCalled();
  });
});
