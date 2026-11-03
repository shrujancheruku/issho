// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { fireEvent, render, screen } from '@testing-library/react-native';
import { Text } from 'react-native';

import { ScreenFrame } from '../screen-frame';

jest.mock('@/i18n/i18n-provider', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
jest.mock('expo-router', () => ({ router: { canGoBack: jest.fn(), back: jest.fn(), replace: jest.fn() } }));
jest.mock('@/constants/theme', () => ({ FormWidth: 600, MaxContentWidth: 800, Spacing: new Proxy({}, { get: () => 4 }) }));
jest.mock('@/hooks/use-theme', () => ({ useTheme: () => new Proxy({}, { get: () => '#000' }) }));
jest.mock('@/components/themed-view', () => ({ ThemedView: ({ children }: { children: unknown }) => children }));
jest.mock('@/components/language-switch', () => {
  const { Text: RNText } = jest.requireActual('react-native');
  return { LanguageSwitch: () => <RNText>language-switch</RNText> };
});
jest.mock('react-native-safe-area-context', () => ({
  SafeAreaView: ({ children }: { children: unknown }) => children,
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
}));
jest.mock('@/components/text-link', () => {
  const { Pressable, Text: RNText } = jest.requireActual('react-native');
  return {
    BackLink: ({ label, onPress }: { label: string; onPress: () => void }) => (
      <Pressable onPress={onPress}>
        <RNText>{label}</RNText>
      </Pressable>
    ),
  };
});

describe('ScreenFrame', () => {
  it('has the back link and the language switch, and the body', async () => {
    const back = jest.fn();
    await render(
      <ScreenFrame onBack={back}>
        <Text>body</Text>
      </ScreenFrame>,
    );
    expect(screen.getByText('language-switch')).toBeTruthy();
    expect(screen.getByText('body')).toBeTruthy();
    await fireEvent.press(screen.getByText('common.back'));
    expect(back).toHaveBeenCalledTimes(1);
  });

  it("puts a tab's own header in place of the usual one", async () => {
    await render(
      <ScreenFrame tab wide header={<Text>tab title</Text>}>
        <Text>body</Text>
      </ScreenFrame>,
    );
    expect(screen.getByText('tab title')).toBeTruthy();
    expect(screen.queryByText('common.back')).toBeNull();
  });

  it('keeps the back link, the language switch and the extra controls on the accent band (a crew)', async () => {
    await render(
      <ScreenFrame accent onBack={jest.fn()} end={<Text>more</Text>} header={<Text>crew name</Text>}>
        <Text>body</Text>
      </ScreenFrame>,
    );
    for (const text of ['common.back', 'language-switch', 'more', 'crew name', 'body']) {
      expect(screen.getByText(text)).toBeTruthy();
    }
  });
});
