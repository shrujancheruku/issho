// What the journeys need from the local stack `make e2e` starts (scripts/e2e.sh):
// the API's test-only clock and forming RPCs, the mail catcher, Auth, and the
// seed that admits the owner. Every address comes from the environment the
// script sets; nothing has a default, so a run by hand fails and says why.
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';

import { expect } from '@playwright/test';

const run = promisify(execFile);

export function env(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set; run the end-to-end tests with \`make e2e\``);
  return value;
}

export const apiOrigin = () => `http://127.0.0.1:${env('E2E_API_PORT')}`;
export const appOrigin = () => `http://127.0.0.1:${env('E2E_APP_PORT')}`;

/** One Connect call, in its JSON form, straight to the API (a Node client has no CORS). */
export async function call<T>(service: string, method: string, body: object = {}, token?: string): Promise<T> {
  const res = await fetch(`${apiOrigin()}/${service}/${method}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${service}/${method}: ${res.status} ${text}`);
  return JSON.parse(text) as T;
}

/**
 * The API's clock, which the worker in the same process reads too
 * (backend/cmd/api/e2e_on.go). The browsers' clocks are moved by `moveClock` in
 * journey.ts, so a screen that works out a window from the device's time agrees
 * with the server that decides it.
 */
export const serverClock = {
  set: (instant: Date) => call('issho.e2e.v1.ClockService', 'SetTime', { now: instant.toISOString() }),
  get: async () => new Date((await call<{ now: string }>('issho.e2e.v1.ClockService', 'GetTime')).now),
};

/** Inserts the forming job for a plan and returns when the worker has run it. */
export const formPlan = (planId: string) => call('issho.e2e.v1.FormingService', 'FormPlan', { planId });

/** The sign-in code Auth mailed to an address, from the mail catcher's API; undefined until the mail is there. */
async function mailedCode(email: string): Promise<string | undefined> {
  const mail = env('E2E_MAIL_URL');
  const found = (await (await fetch(`${mail}/api/v1/search?query=${encodeURIComponent(`to:${email}`)}`)).json()) as { messages?: { ID: string }[] };
  const id = found.messages?.[0]?.ID;
  if (!id) return undefined;
  const message = (await (await fetch(`${mail}/api/v1/message/${id}`)).json()) as { Text: string };
  return message.Text.match(/\b(\d{6})\b/)?.[1];
}

/** The sign-in code Auth mails to an address: asked of the catcher until it has arrived (a poll with a deadline, not a sleep). */
export async function signInCode(email: string): Promise<string> {
  await expect.poll(() => mailedCode(email), { message: `the sign-in code for ${email}`, timeout: 15_000 }).toMatch(/^\d{6}$/);
  return (await mailedCode(email))!;
}

/** Signs an address in through Auth's own API (no browser) and returns its access token. */
export async function signInWithoutBrowser(email: string): Promise<string> {
  const auth = env('E2E_AUTH_URL');
  const json = { 'Content-Type': 'application/json' };
  const sent = await fetch(`${auth}/otp`, { method: 'POST', headers: json, body: JSON.stringify({ email, create_user: true }) });
  if (!sent.ok) throw new Error(`Auth refused to send a code: ${sent.status}`);
  const code = await signInCode(email);
  const verified = await fetch(`${auth}/verify`, { method: 'POST', headers: json, body: JSON.stringify({ email, token: code, type: 'email' }) });
  if (!verified.ok) throw new Error(`Auth refused the code: ${verified.status}`);
  return ((await verified.json()) as { access_token: string }).access_token;
}

/** Loads the fixture again (which replaces each plan whole, signups with it) and admits the account that signed in with this email, as `make seed ADMIT=` does. */
export async function seed(admitEmail?: string): Promise<void> {
  await run(env('E2E_SEED_BINARY'), admitEmail ? ['--admit', admitEmail] : [], { env: { ...process.env, DATABASE_URL: env('E2E_DATABASE_URL') } });
}
