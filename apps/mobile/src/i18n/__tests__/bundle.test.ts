/// <reference types="jest" />
import { fromBundle } from '../bundle';
import { bundled } from '../bundled';
import { argumentNames, english } from '../strings';

const placeholders = (s: string) => argumentNames(s);

describe('bundled languages', () => {
  it('only has strings the app still uses, with their placeholders', () => {
    // A key that's gone or a lost placeholder means `make ui-bundle` wasn't
    // re-run after en.json changed (new keys just show English until the
    // first fetch).
    for (const [lang, b] of Object.entries(bundled)) {
      for (const [key, text] of Object.entries(b.strings)) {
        expect([lang, key, key in english]).toEqual([lang, key, true]);
        expect([lang, key, placeholders(text)]).toEqual([lang, key, placeholders(english[key as keyof typeof english])]);
      }
    }
  });

  it('serves a fresh install, then lets the first fetch replace it', () => {
    const lang = Object.keys(bundled)[0];
    const c = fromBundle(lang);
    expect(c?.complete).toBe(true);
    expect(c?.checkedAt).toBe(0);
    expect(c?.appCatalog).toBeUndefined(); // not "current": fetched at once
    expect(fromBundle('xx')).toBeUndefined();
  });
});
