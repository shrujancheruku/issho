/**
 * Digits in the emailed sign-in code. It belongs to Supabase Auth, not our
 * API, so it can't come from a proto rule: `[auth.email] otp_length` in
 * supabase/config.toml sets it, and lib/__tests__/otp.test.ts checks they match.
 */
export const CODE_LENGTH = 6;
