// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { Code, ConnectError } from '@issho/api-client';
import { fireEvent, render, screen } from '@testing-library/react-native';
import { Text } from 'react-native';

import { QueryScreen, type QueryLike } from '../query-screen';

jest.mock('@/i18n/i18n-provider', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
jest.mock('expo-router', () => ({ router: { canGoBack: jest.fn(), back: jest.fn(), replace: jest.fn() } }));
jest.mock('@/constants/theme', () => ({ FormWidth: 600, Spacing: new Proxy({}, { get: () => 4 }) }));
// Styling and plain components are out of scope here (the theme module loads CSS).
jest.mock('@/components/themed-view', () => ({ ThemedView: ({ children }: { children: unknown }) => children }));
jest.mock('@/components/language-switch', () => ({ LanguageSwitch: () => null }));
jest.mock('react-native-safe-area-context', () => ({ SafeAreaView: ({ children }: { children: unknown }) => children }));
jest.mock('@/components/themed-text', () => {
  const { Text: RNText } = jest.requireActual('react-native');
  return { ThemedText: ({ children }: { children: unknown }) => <RNText>{children}</RNText> };
});
jest.mock('@/components/text-link', () => {
  const { Pressable, Text: RNText } = jest.requireActual('react-native');
  return {
    BackLink: ({ label, onPress }: { label: string; onPress: () => void }) => (
      <Pressable onPress={onPress}>
        <RNText>{label}</RNText>
      </Pressable>
    ),
    TextLink: ({ label, onPress }: { label: string; onPress: () => void }) => (
      <Pressable onPress={onPress}>
        <RNText>{label}</RNText>
      </Pressable>
    ),
  };
});

const pending = (): QueryLike => ({ isSuccess: false, isError: false, error: null, data: undefined, refetch: jest.fn() });
const failed = (error: unknown = new Error('down')): QueryLike => ({ ...pending(), isError: true, error });
const loaded = (data: unknown): QueryLike => ({ ...pending(), isSuccess: true, data });

async function renderScreen(queries: readonly QueryLike[], notFound?: string) {
  return await render(
    <QueryScreen queries={queries} notFound={notFound} onBack={jest.fn()}>
      {(data) => <Text>{JSON.stringify(data)}</Text>}
    </QueryScreen>,
  );
}

// The spinner is a host component with no role or text to query by.
const spinning = () => JSON.stringify(screen.toJSON()).includes('"ActivityIndicator"');

describe('QueryScreen', () => {
  it('shows a spinner while a query loads, and no children', async () => {
    await renderScreen([loaded('a'), pending()]);
    expect(spinning()).toBe(true);
    expect(screen.queryByText('["a"]')).toBeNull();
  });

  it('retries only the queries that failed', async () => {
    const ok = loaded('a');
    const bad = failed();
    await renderScreen([ok, bad]);
    expect(screen.getByText('common.loadFailed')).toBeTruthy();
    await fireEvent.press(screen.getByText('common.retry'));
    expect(bad.refetch).toHaveBeenCalledTimes(1);
    expect(ok.refetch).not.toHaveBeenCalled();
  });

  it('retries every failed query', async () => {
    const a = failed();
    const b = failed();
    await renderScreen([a, b]);
    await fireEvent.press(screen.getByText('common.retry'));
    expect(a.refetch).toHaveBeenCalledTimes(1);
    expect(b.refetch).toHaveBeenCalledTimes(1);
  });

  it('says not found when a query was not found', async () => {
    await renderScreen([loaded('a'), failed(new ConnectError('gone', Code.NotFound))], 'plan.notFound');
    expect(screen.getByText('plan.notFound')).toBeTruthy();
    expect(screen.queryByText('common.retry')).toBeNull();
  });

  it('retries a not-found error when the screen has no not-found message', async () => {
    await renderScreen([failed(new ConnectError('gone', Code.NotFound))]);
    expect(screen.getByText('common.retry')).toBeTruthy();
  });

  it('renders the children with the data once every query succeeded', async () => {
    await renderScreen([loaded({ id: 1 }), loaded('b')]);
    expect(screen.getByText('[{"id":1},"b"]')).toBeTruthy();
  });
});
