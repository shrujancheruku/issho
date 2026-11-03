#!/usr/bin/env node
// Writes the TypeScript apps' coverage totals as a markdown table, from the
// coverage-summary.json that Jest (mobile) and Vitest (admin) write with
// `pnpm test:coverage`. Reported only, never gated (#192). An app that did not
// run (turbo --affected skipped it) is left out.
import { existsSync, readFileSync } from 'node:fs';

const rows = [];
for (const app of ['mobile', 'admin']) {
  const file = `apps/${app}/coverage/coverage-summary.json`;
  if (!existsSync(file)) continue;
  const t = JSON.parse(readFileSync(file, 'utf8')).total;
  const pct = (k) => `${t[k].pct}% (${t[k].covered}/${t[k].total})`;
  rows.push(`| ${app} | ${pct('statements')} | ${pct('branches')} | ${pct('functions')} | ${pct('lines')} |`);
}
if (rows.length) {
  console.log('### TypeScript coverage\n');
  console.log('| App | Statements | Branches | Functions | Lines |');
  console.log('| --- | --- | --- | --- | --- |');
  console.log(rows.join('\n'));
}
