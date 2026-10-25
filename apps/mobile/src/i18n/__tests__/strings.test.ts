/// <reference types="jest" />
import {
  baseLanguage,
  displayEnglish,
  english,
  englishHuman,
  fingerprint,
  format,
  lookup,
  argumentNames,
  pseudo,
  pseudoEnglish,
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
    const placeholders = (s: string) => argumentNames(s);
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

describe('fingerprint', () => {
  it('changes when a text changes, not when keys are reordered', () => {
    const a = fingerprint({ 'common.save': 'Save', 'common.back': 'Back' });
    expect(fingerprint({ 'common.back': 'Back', 'common.save': 'Save' })).toBe(a);
    expect(fingerprint({ 'common.save': 'Save it', 'common.back': 'Back' })).not.toBe(a);
  });
});

describe('catalogue', () => {
  it('uses only {name} placeholders and plural blocks, which survive the server check', () => {
    for (const [key, text] of Object.entries(english)) {
      expect([key, argumentNames(text) !== undefined]).toEqual([key, true]);
    }
  });
});

describe('plurals', () => {
  const friends = english['plan.friends'];
  it('picks the form by the language’s rules', () => {
    expect(format(friends, { count: 1 })).toBe('Someone you get on with is interested');
    expect(format(friends, { count: 3 })).toBe('3 people you get on with are interested');
    const pl = '{count, plural, one {# osoba} few {# osoby} many {# osób} other {# osoby}}';
    expect([1, 3, 5, 22].map((n) => format(pl, { count: n }, 'pl'))).toEqual(['1 osoba', '3 osoby', '5 osób', '22 osoby']);
    expect(format('{count, plural, =0 {Nobody} other {#人}}', { count: 0 }, 'ja')).toBe('Nobody');
    expect(format('{count, plural, other {#人} } and {name}', { count: 2, name: 'A' }, 'ja')).toBe('2人 and A');
  });
  it('parses arguments like the API', () => {
    expect(argumentNames('{b} and {a}')).toEqual(['a', 'b']);
    expect(argumentNames(friends)).toEqual(['count']);
    expect(argumentNames('{count, plural, one {x}')).toBeUndefined();
    expect(argumentNames('{{name}}')).toBeUndefined();
  });
});

it('reduces tags to the base language', () => {
  expect(baseLanguage('ja-JP')).toBe('ja');
  expect(baseLanguage('zh_Hant_TW')).toBe('zh');
  expect(baseLanguage(null)).toBe('');
});

describe('pseudo', () => {
  it('accents, lengthens and brackets text but keeps placeholders', () => {
    const p = pseudo('Signed in as {email}');
    expect(p.startsWith('[Šígñéd íñ áš {email}')).toBe(true);
    expect(p.endsWith(']')).toBe(true);
    expect(p.length).toBeGreaterThan('Signed in as {email}'.length * 1.3);
    expect(pseudoEnglish['common.save']).toMatch(/^\[Šávé ~\]$/);
  });
});
