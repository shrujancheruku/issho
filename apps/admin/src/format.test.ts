import { describe, expect, it } from 'vitest';

import { leanSummary, placeholderProblems } from './format';

describe('leanSummary', () => {
  it('describes a lean', () => {
    const json = JSON.stringify({ gender: 'woman', picked: 9, picks: 10, met: 20, met_share: 0.45, p_value: 0.0021 });
    expect(leanSummary(json)).toBe('9 of 10 meet-again picks were women, who were 45% of the 20 people they met (p = 0.002).');
  });
  it('shows tiny p-values as a bound', () => {
    const json = JSON.stringify({ gender: 'man', picked: 8, picks: 8, met: 30, met_share: 0.3, p_value: 0.00001 });
    expect(leanSummary(json)).toContain('(p = < 0.001)');
  });
  it('falls back to the raw details', () => {
    expect(leanSummary('{"other":1}')).toBe('{"other":1}');
    expect(leanSummary('not json')).toBe('not json');
  });
});

describe('placeholderProblems', () => {
  it('reads plural blocks as their count', () => {
    const en = '{count, plural, one {Someone} other {# people}} near {place}';
    expect(placeholderProblems(en, '{place}の近くに{count, plural, other {#人}}')).toEqual([]);
    expect(placeholderProblems(en, '{place}の近くに#人')).toEqual(['missing {count}']);
    expect(placeholderProblems(en, '{count, plural, other {#人}')).toEqual(["braces { } that don't match up"]);
  });

  it('accepts reordered placeholders', () => {
    expect(placeholderProblems('{a} and {b}', '{b} y {a}')).toEqual([]);
  });
  it('reports missing and unexpected ones', () => {
    expect(placeholderProblems('Code sent to {email}', 'Code envoyé à {courriel}')).toEqual(['missing {email}', 'unexpected {courriel}']);
    expect(placeholderProblems('Hello', 'Hola')).toEqual([]);
  });
});

describe('localParts', () => {
  it('shows a moment in the city, not the browser', async () => {
    const { localParts } = await import('./format');
    const { timestampFromDate } = await import('@bufbuild/protobuf/wkt');
    // 05:30 UTC is 14:30 in Tokyo.
    const ts = timestampFromDate(new Date(Date.UTC(2030, 0, 12, 5, 30)));
    expect(localParts(ts, 'Asia/Tokyo')).toEqual({ date: '2030-01-12', time: '14:30' });
    expect(localParts(undefined, 'Asia/Tokyo')).toEqual({ date: '', time: '' });
  });
});
