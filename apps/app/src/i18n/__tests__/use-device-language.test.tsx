import { renderToString } from 'react-dom/server';
import { render, screen } from '@testing-library/react-native';
import { Text } from 'react-native';

import { useDeviceLanguage } from '../use-device-language';

jest.mock('expo-localization', () => ({ getLocales: jest.fn() }));
const getLocales = jest.mocked(jest.requireMock('expo-localization').getLocales);

function Probe() {
  return <Text>{useDeviceLanguage()}</Text>;
}

describe('useDeviceLanguage', () => {
  it('is the first device language on the list', async () => {
    getLocales.mockReturnValue([{ languageTag: 'fr-FR' }, { languageTag: 'vi-VN' }]);
    await render(<Probe />);
    expect(screen.getByText('vi')).toBeTruthy();
  });

  it('is English for a device set to a language not on the list', async () => {
    getLocales.mockReturnValue([{ languageTag: 'ja-JP' }]);
    await render(<Probe />);
    expect(screen.getByText('en')).toBeTruthy();
  });

  // The server cannot know the visitor's language, so it and the first client
  // render both say English; a mismatch there made React discard the page.
  it('is English on the server whatever the device says', () => {
    getLocales.mockReturnValue([{ languageTag: 'vi-VN' }]);
    expect(renderToString(<Probe />)).toContain('en');
  });
});
