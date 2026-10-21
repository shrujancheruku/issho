/// <reference types="jest" />
import { Gender, Interest, TimeBlock } from '@issho/api-client';

import {
  addSpoken,
  cycleInterest,
  emptyDraft,
  setCity,
  setGender,
  setPrimaryLanguage,
  stepValid,
  toggleSlot,
  toProfile,
  updateSpoken,
  type Draft,
} from '../draft';

const year = 2026;

function complete(): Draft {
  let d = emptyDraft('ja');
  d = { ...d, displayName: ' Aiko ', birthYear: '1995', gender: Gender.WOMAN, cityId: 'tokyo', homeAreaId: 'tokyo-kichijoji' };
  d = toggleSlot(d, 6, TimeBlock.MORNING);
  for (const id of ['hike', 'karaoke', 'pottery']) d = cycleInterest(d, id);
  return { ...d, conductAccepted: true };
}

it('starts in the device language, spoken fluently', () => {
  const d = emptyDraft('es');
  expect(d.primaryLanguage).toBe('es');
  expect(d.spoken).toEqual([{ code: 'es', fluent: true, level: 0 }]);
  expect(stepValid('languages', d)).toBe(true);
  expect(stepValid('about', d, year)).toBe(false);
});

it('cycles interest none → open → like → love → none', () => {
  let d = emptyDraft('en');
  const seen = [];
  for (let i = 0; i < 4; i++) {
    d = cycleInterest(d, 'hike');
    seen.push(d.activities.hike);
  }
  expect(seen).toEqual([Interest.OPEN_TO_TRY, Interest.LIKE, Interest.LOVE, undefined]);
  expect('hike' in d.activities).toBe(false);
});

it('toggles slots', () => {
  const d = toggleSlot(toggleSlot(emptyDraft('en'), 1, TimeBlock.EVENING), 1, TimeBlock.EVENING);
  expect(d.availability).toEqual([]);
});

it('checks age, name and gender on the about step', () => {
  const d = complete();
  expect(stepValid('about', d, year)).toBe(true);
  expect(stepValid('about', { ...d, birthYear: String(year - 17) }, year)).toBe(false);
  expect(stepValid('about', { ...d, birthYear: '95' }, year)).toBe(false);
  expect(stepValid('about', { ...d, displayName: '  ' }, year)).toBe(false);
  expect(stepValid('about', { ...d, gender: Gender.UNSPECIFIED }, year)).toBe(false);
});

it('needs three activities, a fluent language and accepted conduct', () => {
  const d = complete();
  expect(stepValid('activities', d)).toBe(true);
  // hike is at "open to try": three more taps take it off.
  expect(stepValid('activities', cycleInterest(cycleInterest(cycleInterest(d, 'hike'), 'hike'), 'hike'))).toBe(false);
  expect(stepValid('languages', updateSpoken(d, 'ja', { fluent: false }))).toBe(false);
  expect(stepValid('conduct', { ...d, conductAccepted: false })).toBe(false);
});

it('gives learners a level and fluent speakers none', () => {
  let d = addSpoken(emptyDraft('ja'), 'es');
  d = updateSpoken(d, 'es', { fluent: false });
  expect(d.spoken[1]).toEqual({ code: 'es', fluent: false, level: 1 });
  d = updateSpoken(d, 'es', { level: 3 });
  d = updateSpoken(d, 'es', { fluent: true });
  expect(d.spoken[1]).toEqual({ code: 'es', fluent: true, level: 0 });
  expect(addSpoken(d, 'es').spoken).toHaveLength(2);
});

it('drops a secondary language that becomes the primary', () => {
  const d = setPrimaryLanguage({ ...emptyDraft('ja'), secondaryLanguage: 'es' }, 'es');
  expect([d.primaryLanguage, d.secondaryLanguage]).toEqual(['es', '']);
});

it('clears same-gender-only without a gender, and city choices with a new city', () => {
  const d = setGender({ ...complete(), sameGenderOnly: true }, Gender.PREFER_NOT_TO_SAY);
  expect(d.sameGenderOnly).toBe(false);
  const moved = setCity(complete(), 'austin');
  expect([moved.homeAreaId, Object.keys(moved.activities).length]).toEqual(['', 0]);
});

it('builds the SaveProfile payload', () => {
  const p = toProfile(complete(), 1);
  expect(p.displayName).toBe('Aiko');
  expect(p.birthYear).toBe(1995);
  expect(p.availability).toEqual([{ weekday: 6, block: TimeBlock.MORNING }]);
  expect(p.activities).toHaveLength(3);
  expect(p.conductVersion).toBe(1);
});
