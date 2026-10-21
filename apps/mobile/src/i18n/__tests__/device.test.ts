/// <reference types="jest" />
import { deviceLanguages } from '../device';

const l = (...tags: string[]) => tags.map((t) => ({ languageCode: t.split('-')[0], languageTag: t }));

it('takes the first two supported languages, most preferred first', () => {
  expect(deviceLanguages(l('ja-JP', 'en-US', 'es-ES'))).toEqual({ primary: 'ja', secondary: 'en' });
  expect(deviceLanguages(l('pt-BR'))).toEqual({ primary: 'pt', secondary: '' });
});

it('skips regional duplicates and unsupported languages', () => {
  expect(deviceLanguages(l('en-GB', 'en-US', 'haw-US', 'ko-KR'))).toEqual({ primary: 'en', secondary: 'ko' });
});

it('maps codes the server has under another name', () => {
  expect(deviceLanguages(l('fil-PH', 'en-PH'))).toEqual({ primary: 'tl', secondary: 'en' });
  expect(deviceLanguages(l('yue-Hant-HK'))).toEqual({ primary: 'zh', secondary: '' });
  expect(deviceLanguages(l('iw-IL'))).toEqual({ primary: 'he', secondary: '' });
});

it('falls back to English', () => {
  expect(deviceLanguages([])).toEqual({ primary: 'en', secondary: '' });
  expect(deviceLanguages(l('haw-US'))).toEqual({ primary: 'en', secondary: '' });
  expect(deviceLanguages([{ languageCode: null, languageTag: 'de-DE' }])).toEqual({ primary: 'de', secondary: '' });
});
