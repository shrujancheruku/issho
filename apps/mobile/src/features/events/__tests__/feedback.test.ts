/// <reference types="jest" />
import type { FeedbackAnswer } from '../data';
import { submittedAnswer, toggleMate } from '../feedback';

const base: FeedbackAnswer = { attended: true, planRating: 4, groupRating: 5, picks: [], avoid: [] };

describe('toggleMate', () => {
  it('adds a pick, and toggling it again removes it', () => {
    const on = toggleMate(base, 'picks', 'a');
    expect(on.picks).toEqual(['a']);
    expect(toggleMate(on, 'picks', 'a').picks).toEqual([]);
  });

  it('adds to avoid the same way', () => {
    expect(toggleMate(base, 'avoid', 'a').avoid).toEqual(['a']);
  });

  it('picking someone already avoided moves them, so they are never in both', () => {
    const r = toggleMate({ ...base, avoid: ['a', 'b'] }, 'picks', 'a');
    expect(r.picks).toEqual(['a']);
    expect(r.avoid).toEqual(['b']);
  });

  it('avoiding someone already picked moves them', () => {
    const r = toggleMate({ ...base, picks: ['a', 'b'] }, 'avoid', 'a');
    expect(r.avoid).toEqual(['a']);
    expect(r.picks).toEqual(['b']);
  });

  it('leaves others and the ratings alone, and does not mutate its input', () => {
    const before: FeedbackAnswer = { ...base, picks: ['b'], avoid: ['c'] };
    const frozen = JSON.stringify(before);
    const r = toggleMate(before, 'picks', 'a');
    expect(r).toMatchObject({ planRating: 4, groupRating: 5, attended: true, picks: ['b', 'a'], avoid: ['c'] });
    expect(JSON.stringify(before)).toBe(frozen);
  });

  it('no sequence of toggles leaves anyone in both lists', () => {
    let a = base;
    for (const [list, mate] of [['picks', 'x'], ['avoid', 'x'], ['picks', 'y'], ['picks', 'x'], ['avoid', 'y'], ['avoid', 'x']] as const) {
      a = toggleMate(a, list, mate);
      expect(a.picks.filter((m) => a.avoid.includes(m))).toEqual([]);
    }
  });
});

describe('submittedAnswer', () => {
  it('sends the answer as is when they went', () => {
    const a = { ...base, picks: ['a'], avoid: ['b'] };
    expect(submittedAnswer(a)).toEqual(a);
  });

  it('sends only "did not go" when they did not, dropping ratings and picks', () => {
    expect(submittedAnswer({ ...base, attended: false, picks: ['a'], avoid: ['b'] })).toEqual({
      attended: false,
      planRating: 0,
      groupRating: 0,
      picks: [],
      avoid: [],
    });
  });
});
