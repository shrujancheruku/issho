/// <reference types="jest" />
import { crewName, withoutExamples } from '../name';

describe('crewName', () => {
  const names = {
    activity: (id: string) => (id === 'food-tour' ? { name: 'Food tour (ramen, desserts …)', secondary: '食べ歩き（ラーメン）' } : { name: id, secondary: '' }),
    area: (id: string) => (id === 'kichijoji' ? { name: 'Kichijoji', secondary: '吉祥寺' } : { name: id, secondary: '' }),
  };
  const template = (name: string) => `${name} crew`;

  it('names an activity crew after the activity, without examples', () => {
    expect(crewName({ activityId: 'food-tour', areaId: '' }, names, template)).toEqual({ name: 'Food tour crew', secondary: '食べ歩き（ラーメン） crew' });
  });

  it('names an area crew after the area, in both stored names', () => {
    expect(crewName({ activityId: '', areaId: 'kichijoji' }, names, template)).toEqual({ name: 'Kichijoji crew', secondary: '吉祥寺 crew' });
  });

  it('leaves names without examples alone', () => {
    expect(withoutExamples('Casual soccer game')).toBe('Casual soccer game');
  });
});
