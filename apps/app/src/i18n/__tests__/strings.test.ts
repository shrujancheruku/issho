import { fill, pickLanguage, translate } from '../strings';

describe('translate', () => {
  it('returns the text in the display language', () => {
    expect(translate('en', 'tabs.plans')).toBe('Plans');
    expect(translate('vi', 'tabs.plans')).toBe('Kế hoạch');
  });

  it('fills {placeholders} and leaves an unnamed one as written', () => {
    expect(fill('Signed in as {name}', { name: 'Linh' })).toBe('Signed in as Linh');
    expect(fill('Hello {name}')).toBe('Hello {name}');
  });
});

describe('pickLanguage', () => {
  it('takes the first device language on the list', () => {
    expect(pickLanguage(['fr-FR', 'vi-VN', 'en-US'])).toBe('vi');
  });

  it('gives English to a device set to a language not on the list', () => {
    expect(pickLanguage(['ja-JP'])).toBe('en');
    expect(pickLanguage([])).toBe('en');
  });
});
