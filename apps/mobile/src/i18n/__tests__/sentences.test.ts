/// <reference types="jest" />
import { english, format, type StringKey } from '../strings';

// The sentences that used to be glued from t() pieces and punctuation are now
// one template each. The English they render must not change: each case holds
// the old composition (written out from the old pieces) beside the template.
const t = (key: StringKey, params?: Record<string, string | number>) => format(english[key], params);

describe('whole-sentence templates render the English they replaced', () => {
  const area = 'Shibuya · 渋谷';
  const cases: [string, string, string][] = [
    ['area, interested', `${area}. Interested: 4`, t('plan.areaInterested', { area, count: 4 })],
    ['area, last day', `${area}. Last day to sign up: Fri 6 Nov`, t('plan.areaSignupBy', { area, date: 'Fri 6 Nov' })],
    ['last day', 'Last day to sign up: Fri 6 Nov', t('plan.signupByDate', { date: 'Fri 6 Nov' })],
    ['area, distance', `${area}, 20 min away`, t('plan.areaDistance', { area, distance: t('plan.minutes', { minutes: 20 }) })],
    [
      'area, distance, language',
      `${area}, 20 min away. Japanese`,
      t('plan.areaDistanceLanguage', { area, distance: t('plan.minutes', { minutes: 20 }), language: 'Japanese' }),
    ],
    [
      'interested, when',
      'Interested: 4. No date yet. We pick one when enough people can go at the same time.',
      t('plan.interestedWhen', { count: 4 }),
    ],
    ['slot', 'Wed Evening', t('plan.slot', { day: 'Wed', block: 'Evening' })],
    [
      'area, travel',
      `${area}. Longest trip you'll make: 30 min`,
      t('profile.areaTravel', { area, minutes: t('profile.minutes', { count: 30 }) }),
    ],
    ['group size', 'Group size: Small', t('profile.groupSizeValue', { size: 'Small' })],
    ['one language', 'Main language: Japanese', t('you.languageMain', { language: 'Japanese' })],
    [
      'two languages',
      'Main language: Japanese. Second language: English',
      t('you.languagesBoth', { primary: 'Japanese', secondary: 'English' }),
    ],
    ['meeting point', 'Meeting point: Hachiko', t('event.meetingPointLine', { place: 'Hachiko' })],
    ['going', 'Who is going: Ann, Bo', t('event.goingLine', { names: 'Ann, Bo' })],
    ['said yes', 'Ann, said yes', t('crew.personSaidYes', { name: 'Ann' })],
    ['deciding', 'Ann, deciding', t('crew.personDeciding', { name: 'Ann' })],
    ['minimum, picked', 'Choose at least 3 activities. 2 picked', t('interest.minimumPicked', { min: 3, picked: 2 })],
  ];
  it.each(cases)('%s', (_name, before, after) => {
    expect(after).toBe(before);
  });
});
