import { Code, ConnectError } from '@connectrpc/connect';
import { OutingState, SeatState } from '@issho/api-client';

import { anOuting } from '@/test-support/outing-fixtures';

import { checkInStage, feedbackRefusalOf, outingOver, refusalOf } from '../model';

const refused = new ConnectError('refused', Code.FailedPrecondition);
const going = anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET });
// Check-in is 01:30 to 05:30 UTC on 25 Nov; feedback is 05:30 UTC on 25 Nov to 05:30 UTC on 2 Dec.
const at = (iso: string) => new Date(iso);

describe('why a refusal was a refusal, from the outing\'s own windows', () => {
  it.each([
    ['too early', '2026-11-25T01:29:59Z', 'checkInEarly'],
    ['the outing is over', '2026-11-25T05:30:00Z', 'checkInOver'],
    ['inside the window, for a reason the screen cannot know', '2026-11-25T03:00:00Z', 'checkInFailed'],
  ])('a refused check-in %s', (_name, now, refusal) => {
    expect(refusalOf(refused, 'checkIn', going, at(now))).toBe(refusal);
  });

  it('says a check-in that failed any other way is a plain failure', () => {
    expect(refusalOf(new ConnectError('down', Code.Unavailable), 'checkIn', going, at('2026-11-25T03:00:00Z'))).toBe('checkInFailed');
    expect(refusalOf(refused, 'checkIn', undefined, at('2026-11-25T03:00:00Z'))).toBe('checkInFailed');
  });

  it.each([
    ['before it opens', '2026-11-25T05:29:59Z', 'notOpen'],
    ['once it has closed', '2026-12-02T05:30:00Z', 'closed'],
    ['inside the window, for a reason the screen cannot know', '2026-11-28T00:00:00Z', 'failed'],
  ])('a refused feedback save %s', (_name, now, refusal) => {
    expect(feedbackRefusalOf(refused, going, at(now))).toBe(refusal);
  });

  it('says a save that failed any other way is a plain failure', () => {
    expect(feedbackRefusalOf(new ConnectError('down', Code.Unavailable), going, at('2026-12-03T00:00:00Z'))).toBe('failed');
  });
});

describe('the windows', () => {
  it('opens check-in at its first instant and closes it at the end', () => {
    expect(checkInStage(going, at('2026-11-25T01:29:59Z'))).toBe('early');
    expect(checkInStage(going, at('2026-11-25T01:30:00Z'))).toBe('open');
    expect(checkInStage(going, at('2026-11-25T05:29:59Z'))).toBe('open');
    expect(checkInStage(going, at('2026-11-25T05:30:00Z'))).toBe('closed');
  });

  it('is over from the plan\'s end, and not before', () => {
    expect(outingOver(going, at('2026-11-25T05:29:59Z'))).toBe(false);
    expect(outingOver(going, at('2026-11-25T05:30:00Z'))).toBe(true);
  });
});
