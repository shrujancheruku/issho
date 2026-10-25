/// <reference types="jest" />
import { isRightToLeft } from '../direction';

it('knows which display languages are written right to left', () => {
  expect(['ar', 'he', 'fa-IR', 'ur'].map(isRightToLeft)).toEqual([true, true, true, true]);
  expect(['en', 'ja', 'zh-Hant', ''].map(isRightToLeft)).toEqual([false, false, false, false]);
});
