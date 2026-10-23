/// <reference types="jest" />
import { walkHours, withoutExamples } from '../format';

it('drops trailing examples in brackets, ASCII or full-width', () => {
  expect(withoutExamples('Food tour (ramen, desserts, markets …)')).toBe('Food tour');
  expect(withoutExamples('グルメツアー（ラーメン、デザート、市場など）')).toBe('グルメツアー');
  expect(withoutExamples('Karaoke')).toBe('Karaoke');
  expect(withoutExamples('Board games (Go) night')).toBe('Board games (Go) night');
});

describe('walkHours', () => {
  const step = (minutes: number, walkMinutes: number) => ({ minutes, walkMinutes });

  it('adds the time at each stop and the walks, to the nearest half hour', () => {
    // 10 + 25 + 25 + 45 at the stops, 5 + 12 + 8 walking: 130 min.
    expect(walkHours([step(10, 0), step(25, 5), step(25, 12), step(45, 8)], 'en')).toBe('2');
    expect(walkHours([step(15, 0), step(60, 10), step(60, 10), step(45, 10)], 'en')).toBe('3.5');
  });

  it('is empty for a plan that is not a walk', () => {
    expect(walkHours([step(0, 0), step(0, 7)], 'en')).toBe('');
    expect(walkHours([], 'en')).toBe('');
  });
});
