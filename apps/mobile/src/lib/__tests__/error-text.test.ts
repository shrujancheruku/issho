/// <reference types="jest" />
import { Code, ConnectError, ErrorReason, ErrorReasonDetailSchema } from '@issho/api-client';

import { errorText } from '../error-text';

const t = (k: string) => k;
const text = (err: unknown) => errorText(err, t as never);

const withReason = (code: Code, reason: ErrorReason, message = 'server words') =>
  new ConnectError(message, code, undefined, [{ desc: ErrorReasonDetailSchema, value: { reason } }]);

it.each([
  [ErrorReason.SUSPENDED, Code.PermissionDenied, 'error.suspended'],
  [ErrorReason.NO_PROFILE, Code.FailedPrecondition, 'error.noProfile'],
  [ErrorReason.EVENT_OVER, Code.FailedPrecondition, 'error.eventOver'],
  [ErrorReason.TOO_LATE, Code.FailedPrecondition, 'event.tooLate'],
  [ErrorReason.NOT_NOW, Code.FailedPrecondition, 'error.notNow'],
  [ErrorReason.CHECK_IN_CLOSED, Code.FailedPrecondition, 'error.checkInClosed'],
  [ErrorReason.FEEDBACK_CLOSED, Code.FailedPrecondition, 'feedback.closed'],
  [ErrorReason.FEATURE_UNAVAILABLE, Code.FailedPrecondition, 'suggest.unavailable'],
])('reason %s reads as %s', (reason, code, key) => {
  expect(text(withReason(code, reason))).toBe(key);
});

it('says the network failed for Unavailable and for a failed fetch', () => {
  expect(text(new ConnectError('down', Code.Unavailable))).toBe('error.network');
  expect(text(new TypeError('Failed to fetch'))).toBe('error.network');
  expect(text(new ConnectError('x', Code.Unknown, undefined, undefined, new TypeError('Network request failed')))).toBe(
    'error.network',
  );
});

it('says something generic for anything else, even a precondition without a reason', () => {
  expect(text(new Error('boom'))).toBe('error.generic');
  expect(text(new ConnectError('x', Code.Internal))).toBe('error.generic');
  expect(text(new ConnectError('x', Code.FailedPrecondition))).toBe('error.generic');
  expect(text(null)).toBe('error.generic');
});

it('never shows the server message', () => {
  const shown = [
    text(withReason(Code.FailedPrecondition, ErrorReason.TOO_LATE, 'SECRET words')),
    text(new ConnectError('SECRET words', Code.Internal)),
    text(new ConnectError('SECRET words', Code.Unavailable)),
    text(new Error('SECRET words')),
  ];
  for (const m of shown) expect(m).not.toContain('SECRET');
});
