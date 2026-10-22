/// <reference types="jest" />
import { withoutExamples } from '../format';

it('drops trailing examples in brackets, ASCII or full-width', () => {
  expect(withoutExamples('Food tour (ramen, desserts, markets …)')).toBe('Food tour');
  expect(withoutExamples('グルメツアー（ラーメン、デザート、市場など）')).toBe('グルメツアー');
  expect(withoutExamples('Karaoke')).toBe('Karaoke');
  expect(withoutExamples('Board games (Go) night')).toBe('Board games (Go) night');
});
