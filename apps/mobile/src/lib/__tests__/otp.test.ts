/// <reference types="jest" />
import { CODE_LENGTH } from '../otp';

// The app has no Node types; jest runs in Node, so declare the two things used.
declare const __dirname: string;
declare function require(id: string): any;
const { readFileSync } = require('fs') as { readFileSync(path: string, encoding: string): string };

// The sign-in code length is Supabase Auth's setting, not ours: if the two
// drift apart, the sign-in box either cuts the code short or never submits.
it('CODE_LENGTH equals [auth.email] otp_length in supabase/config.toml', () => {
  const config = readFileSync(`${__dirname}/../../../../../supabase/config.toml`, 'utf8');
  // The text of the [auth.email] table: up to the next table header.
  const section = /^\[auth\.email\]\n([\s\S]*?)(?=^\[)/m.exec(config)?.[1];
  const length = /^otp_length\s*=\s*(\d+)/m.exec(section ?? '')?.[1];
  expect(length).toBeDefined();
  expect(CODE_LENGTH).toBe(Number(length));
});
