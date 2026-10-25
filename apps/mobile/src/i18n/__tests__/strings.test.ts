/// <reference types="jest" />
import {
  baseLanguage,
  coversApp,
  displayEnglish,
  english,
  englishHuman,
  fingerprint,
  format,
  lookup,
  type StringKey,
} from '../strings';

describe('format', () => {
  it('fills placeholders and leaves unknown ones', () => {
    expect(format('{a} and {b}', { a: 1 })).toBe('1 and {b}');
    expect(format('no params')).toBe('no params');
  });
});

describe('lookup', () => {
  it('uses the shown language, then the other one, then English', () => {
    const ja = { 'common.save': '保存' };
    const es = { 'common.save': 'Guardar', 'common.back': 'Atrás' };
    expect(lookup('common.save', [ja, es])).toBe('保存');
    expect(lookup('common.back', [ja, es])).toBe('Atrás');
    expect(lookup('common.cancel', [ja, es])).toBe('Cancel');
    expect(lookup('account.signedInAs', [undefined], { email: 'a@b.c' })).toBe('Signed in as a@b.c');
  });
});

describe('human English', () => {
  it('replaces only real keys and keeps their placeholders', () => {
    // The Go tests check the same file in depth (staleness included); this
    // guards what the app bundles.
    const placeholders = (s: string) => (s.match(/\{[a-zA-Z]+\}/g) ?? []).sort();
    for (const [key, v] of Object.entries(englishHuman)) {
      expect(key in english).toBe(true);
      expect([key, placeholders(v!.text)]).toEqual([key, placeholders(english[key as StringKey])]);
      expect(displayEnglish[key as StringKey]).toBe(v!.text);
    }
    expect(Object.keys(displayEnglish).sort()).toEqual(Object.keys(english).sort());
  });

  it('is shown for English but never as the last resort', () => {
    const human = { 'common.save': 'Save it' };
    expect(lookup('common.save', [human])).toBe('Save it');
    expect(lookup('common.save', [undefined])).toBe(english['common.save']);
  });
});

describe('coversApp', () => {
  it('needs every key', () => {
    expect(coversApp(english)).toBe(true);
    expect(coversApp({ 'common.save': 'x' })).toBe(false);
  });
});

describe('fingerprint', () => {
  it('changes when a text changes, not when keys are reordered', () => {
    const a = fingerprint({ 'common.save': 'Save', 'common.back': 'Back' });
    expect(fingerprint({ 'common.back': 'Back', 'common.save': 'Save' })).toBe(a);
    expect(fingerprint({ 'common.save': 'Save it', 'common.back': 'Back' })).not.toBe(a);
  });
});

describe('catalogue', () => {
  it('uses only {name} placeholders, which survive the server check', () => {
    for (const [key, text] of Object.entries(english)) {
      expect([key, (text.match(/\{/g) ?? []).length]).toEqual([key, (text.match(/\{[a-zA-Z]+\}/g) ?? []).length]);
    }
  });
});

it('reduces tags to the base language', () => {
  expect(baseLanguage('ja-JP')).toBe('ja');
  expect(baseLanguage('zh_Hant_TW')).toBe('zh');
  expect(baseLanguage(null)).toBe('');
});
