/// <reference types="jest" />
import { PlanLevel } from '@issho/api-client';
import { render, screen } from '@testing-library/react-native';
import { StyleSheet } from 'react-native';

import { LevelTag } from '../level-tag';

jest.mock('@/i18n/i18n-provider', () => ({ useI18n: () => ({ t: (key: string) => key }) }));
jest.mock('@/constants/theme', () => ({
  Brand: { gold: 'gold', goldTint: 'goldTint', ink: 'ink' },
  Spacing: new Proxy({}, { get: () => 4 }),
}));
// Each colour is its own name, so a test can tell which one was used.
jest.mock('@/hooks/use-theme', () => ({ useTheme: () => new Proxy({}, { get: (_t, key) => String(key) }) }));
jest.mock('@/components/themed-text', () => {
  const { Text } = jest.requireActual('react-native');
  return { ThemedText: Text };
});

const styleOf = (id: string) => StyleSheet.flatten(screen.getByText(id).parent?.props.style);

describe('LevelTag', () => {
  it('colours the interest levels', async () => {
    await render(<LevelTag level={PlanLevel.TOP} />);
    expect(styleOf('interest.love').backgroundColor).toBe('love');
  });

  it('renders nothing for a level without colours, unless plain', async () => {
    await render(<LevelTag level={PlanLevel.LOW} />);
    expect(screen.queryByText('plan.level.low')).toBeNull();
  });

  it('is grey for a level without colours when plain', async () => {
    await render(<LevelTag level={PlanLevel.LOW} plain />);
    expect(styleOf('plan.level.low').backgroundColor).toBe('backgroundSelected');
  });
});
