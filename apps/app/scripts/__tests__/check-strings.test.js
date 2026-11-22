const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { checkStrings } = require('../check-strings');

// A throwaway app: the strings files and one source file that uses some keys.
function app({ en, vi, source }) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'strings-'));
  fs.mkdirSync(path.join(root, 'src/i18n'), { recursive: true });
  fs.writeFileSync(path.join(root, 'src/i18n/en.json'), JSON.stringify(en));
  fs.writeFileSync(path.join(root, 'src/i18n/vi.json'), JSON.stringify(vi));
  fs.writeFileSync(path.join(root, 'src/screen.tsx'), source);
  return root;
}

describe('checkStrings', () => {
  it('passes when both languages hold the same keys and every key is used', () => {
    const root = app({ en: { a: 'A', b: 'B' }, vi: { a: 'Á', b: 'B' }, source: "t('a'); t('b');" });
    expect(checkStrings(root)).toEqual([]);
  });

  it('names a key a language is missing', () => {
    const root = app({ en: { a: 'A', b: 'B' }, vi: { a: 'Á' }, source: "t('a'); t('b');" });
    expect(checkStrings(root)).toEqual(['vi.json is missing "b"']);
  });

  it('names a key a language has that English does not', () => {
    const root = app({ en: { a: 'A' }, vi: { a: 'Á', z: 'Z' }, source: "t('a');" });
    expect(checkStrings(root)).toContain('vi.json has "z", which en.json does not');
  });

  it('names a key no source file uses', () => {
    const root = app({ en: { a: 'A', b: 'B' }, vi: { a: 'Á', b: 'B' }, source: "t('a');" });
    expect(checkStrings(root)).toEqual(['"b" is unused']);
  });

  it('names a key the code asks for that no file holds', () => {
    const root = app({ en: { a: 'A' }, vi: { a: 'Á' }, source: "t('a'); t('nope');" });
    expect(checkStrings(root)).toEqual(['"nope" is used in src/screen.tsx but is not in en.json']);
  });
});
