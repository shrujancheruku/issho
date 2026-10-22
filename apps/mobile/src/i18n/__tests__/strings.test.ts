/// <reference types="jest" />
import { baseLanguage, coversApp, english, fingerprint, format, lookup } from '../strings';

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
