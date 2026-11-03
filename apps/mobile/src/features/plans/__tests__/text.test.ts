import { categoryLabel, distanceText, friendsText, pairText } from '../text';

jest.mock('@/i18n/strings', () => ({ english: { 'category.food': 'Food' } }));

describe('pairText', () => {
  it('puts the secondary text beside the name', () => {
    expect(pairText({ name: 'Shibuya', secondary: '渋谷' })).toBe('Shibuya · 渋谷');
  });

  it('is just the name when there is no secondary', () => {
    expect(pairText({ name: 'Shibuya', secondary: '' })).toBe('Shibuya');
  });
});

describe('distanceText', () => {
  const t = ((key: string, params?: { minutes?: number }) => `${key}:${params?.minutes ?? ''}`) as never;

  it.each([
    [-1, 'plan.noRoute:'],
    [0, 'plan.nearby:'],
    [12, 'plan.minutes:12'],
  ])('%i minutes reads %s', (minutes, expected) => {
    expect(distanceText(t, minutes)).toBe(expected);
  });
});

describe('friendsText', () => {
  it('passes the count', () => {
    const t = ((key: string, params?: { count?: number }) => `${key}:${params?.count}`) as never;
    expect(friendsText(t, 3)).toBe('plan.friends:3');
  });
});

describe('categoryLabel', () => {
  const t = (key: string) => `t(${key})`;

  it('translates a category this build knows', () => {
    expect(categoryLabel(t, 'food')).toBe('t(category.food)');
  });

  it('shows the id of a category added on the server since this build', () => {
    expect(categoryLabel(t, 'karaoke')).toBe('karaoke');
  });
});
