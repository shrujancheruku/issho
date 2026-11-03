/// <reference types="jest" />
import { signInErrorText } from '../sign-in-error';

const t = (k: string) => k;
const text = (e: { code?: string }) => signInErrorText(e, t as never);

it.each([
  ['otp_expired', 'signIn.errorExpired'],
  ['over_email_send_rate_limit', 'signIn.errorRateLimit'],
  ['email_address_invalid', 'signIn.errorInvalid'],
  ['validation_failed', 'signIn.errorInvalid'],
])('maps %s to %s', (code, key) => {
  expect(text({ code })).toBe(key);
});

it('says something generic for an unknown or missing code', () => {
  expect(text({ code: 'something_new' })).toBe('error.generic');
  expect(text({})).toBe('error.generic');
});
