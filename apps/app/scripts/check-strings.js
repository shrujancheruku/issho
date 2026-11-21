// The strings check (docs/spec/quality.md): en.json and vi.json hold the same
// keys, every key appears as a string somewhere under src/, and every key a t() call
// names exists. `make lint` runs it; src/i18n/__tests__ and
// scripts/__tests__ cover it.
const fs = require('node:fs');
const path = require('node:path');

const OTHER_LANGUAGES = ['vi'];

function sourceFiles(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return entry.name === '__tests__' ? [] : sourceFiles(full);
    return /\.(ts|tsx)$/.test(entry.name) ? [full] : [];
  });
}

/** Problems found under `root` (an app directory), one sentence each; empty when clean. */
function checkStrings(root) {
  const read = (lang) => JSON.parse(fs.readFileSync(path.join(root, 'src/i18n', `${lang}.json`), 'utf8'));
  const english = new Set(Object.keys(read('en')));
  const problems = [];

  for (const lang of OTHER_LANGUAGES) {
    const keys = new Set(Object.keys(read(lang)));
    for (const key of english) if (!keys.has(key)) problems.push(`${lang}.json is missing "${key}"`);
    for (const key of keys) if (!english.has(key)) problems.push(`${lang}.json has "${key}", which en.json does not`);
  }

  const files = sourceFiles(path.join(root, 'src')).map((file) => ({ file, text: fs.readFileSync(file, 'utf8') }));
  // A key is used when its name appears as a string anywhere in the source, so a
  // screen can pass a key as a prop (title="tabs.plans") as well as call t('key').
  for (const key of english) {
    if (!files.some(({ text }) => text.includes(`'${key}'`) || text.includes(`"${key}"`))) problems.push(`"${key}" is unused`);
  }
  const asked = new Map(); // key -> first file calling t() with it
  for (const { file, text } of files) {
    for (const [, key] of text.matchAll(/\bt\(\s*['"]([\w.]+)['"]/g)) {
      if (!asked.has(key)) asked.set(key, path.relative(root, file));
    }
  }
  for (const [key, file] of asked) {
    if (!english.has(key)) problems.push(`"${key}" is used in ${file} but is not in en.json`);
  }
  return problems;
}

module.exports = { checkStrings };

if (require.main === module) {
  const problems = checkStrings(path.join(__dirname, '..'));
  for (const problem of problems) console.error(`strings: ${problem}`);
  process.exit(problems.length ? 1 : 0);
}
