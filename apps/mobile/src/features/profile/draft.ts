import { Gender, GroupSize, Interest, TimeBlock, type Profile } from '@issho/api-client';

/**
 * The profile as onboarding and settings edit it. Plain data, converted to and
 * from the API's Profile message at the edges, so steps stay simple and the
 * rules here can be unit-tested.
 */
export type Draft = {
  displayName: string;
  birthYear: string; // as typed
  gender: Gender;
  sameGenderOnly: boolean;
  cityId: string;
  homeAreaId: string;
  maxTravelMin: number;
  spoken: SpokenLanguage[];
  primaryLanguage: string;
  secondaryLanguage: string; // '' = none
  availability: string[]; // slotKey(weekday, block)
  activities: Record<string, Interest>;
  groupSize: GroupSize;
  energy: number;
  budget: number;
  conductAccepted: boolean;
};

export type SpokenLanguage = { code: string; fluent: boolean; level: number };

export const steps = ['languages', 'about', 'area', 'availability', 'activities', 'style', 'conduct'] as const;
export type Step = (typeof steps)[number];

/**
 * The code-of-conduct version whose rules this build shows (ConductStep).
 * Bump it with profile.ConductVersion on the server when the rules change.
 */
export const ConductVersion = 1;

/** Must match the server (internal/profile). */
export const MinActivities = 3;
export const MinAge = 18;
export const travelChoices = [15, 30, 45, 60, 90];
export const blocks = [TimeBlock.MORNING, TimeBlock.AFTERNOON, TimeBlock.EVENING];
/** Monday first; values are 0 = Sunday as on the server. */
export const weekdays = [1, 2, 3, 4, 5, 6, 0];

export function emptyDraft(deviceLanguage: string, deviceSecondary = ''): Draft {
  return {
    displayName: '',
    birthYear: '',
    gender: Gender.UNSPECIFIED,
    sameGenderOnly: false,
    cityId: '',
    homeAreaId: '',
    maxTravelMin: 30,
    spoken: [{ code: deviceLanguage, fluent: true, level: 0 }],
    primaryLanguage: deviceLanguage,
    secondaryLanguage: deviceSecondary === deviceLanguage ? '' : deviceSecondary,
    availability: [],
    activities: {},
    groupSize: GroupSize.MEDIUM,
    energy: 2,
    budget: 1,
    conductAccepted: false,
  };
}

export const slotKey = (weekday: number, block: TimeBlock) => `${weekday}:${block}`;

export function toggleSlot(d: Draft, weekday: number, block: TimeBlock): Draft {
  const key = slotKey(weekday, block);
  const availability = d.availability.includes(key)
    ? d.availability.filter((k) => k !== key)
    : [...d.availability, key];
  return { ...d, availability };
}

/** Tapping a tile steps up: none → open to try → like → love → none. */
export function cycleInterest(d: Draft, activityId: string): Draft {
  const order = [Interest.UNSPECIFIED, Interest.OPEN_TO_TRY, Interest.LIKE, Interest.LOVE];
  const current = d.activities[activityId] ?? Interest.UNSPECIFIED;
  const next = order[(order.indexOf(current) + 1) % order.length];
  const activities = { ...d.activities };
  if (next === Interest.UNSPECIFIED) delete activities[activityId];
  else activities[activityId] = next;
  return { ...d, activities };
}

/** Changing city clears choices that belong to the old one. */
export function setCity(d: Draft, cityId: string): Draft {
  if (cityId === d.cityId) return d;
  return { ...d, cityId, homeAreaId: '', activities: {} };
}

export function setGender(d: Draft, gender: Gender): Draft {
  // Same-gender-only groups need a gender to match on (server rule).
  return { ...d, gender, sameGenderOnly: gender === Gender.PREFER_NOT_TO_SAY ? false : d.sameGenderOnly };
}

export function addSpoken(d: Draft, code: string): Draft {
  if (d.spoken.some((l) => l.code === code)) return d;
  return { ...d, spoken: [...d.spoken, { code, fluent: true, level: 0 }] };
}

export function updateSpoken(d: Draft, code: string, change: Partial<SpokenLanguage>): Draft {
  return {
    ...d,
    spoken: d.spoken.map((l) => {
      if (l.code !== code) return l;
      const next = { ...l, ...change };
      // Learners need a level; fluent speakers have none.
      return { ...next, level: next.fluent ? 0 : next.level || 1 };
    }),
  };
}

export function removeSpoken(d: Draft, code: string): Draft {
  return { ...d, spoken: d.spoken.filter((l) => l.code !== code) };
}

export function setPrimaryLanguage(d: Draft, code: string): Draft {
  return { ...d, primaryLanguage: code, secondaryLanguage: d.secondaryLanguage === code ? '' : d.secondaryLanguage };
}

/** Whether a step's answers are complete enough to continue. */
export function stepValid(step: Step, d: Draft, year = new Date().getFullYear()): boolean {
  switch (step) {
    case 'languages':
      return !!d.primaryLanguage && d.secondaryLanguage !== d.primaryLanguage && d.spoken.some((l) => l.fluent);
    case 'about': {
      const y = Number(d.birthYear);
      const name = d.displayName.trim();
      return (
        name.length > 0 &&
        [...name].length <= 40 &&
        /^\d{4}$/.test(d.birthYear) &&
        y <= year - MinAge &&
        y >= year - 100 &&
        d.gender !== Gender.UNSPECIFIED
      );
    }
    case 'area':
      return !!d.homeAreaId;
    case 'availability':
      return d.availability.length > 0;
    case 'activities':
      return Object.keys(d.activities).length >= MinActivities;
    case 'style':
      return d.groupSize !== GroupSize.UNSPECIFIED;
    case 'conduct':
      return d.conductAccepted;
  }
}

export function fromProfile(p: Profile, currentConductVersion: number): Draft {
  return {
    displayName: p.displayName,
    birthYear: String(p.birthYear),
    gender: p.gender,
    sameGenderOnly: p.sameGenderOnly,
    cityId: p.cityId,
    homeAreaId: p.homeAreaId,
    maxTravelMin: p.maxTravelMin,
    spoken: p.spokenLanguages.map((l) => ({ code: l.code, fluent: l.fluent, level: l.level })),
    primaryLanguage: p.primaryLanguage,
    secondaryLanguage: p.secondaryLanguage,
    availability: p.availability.map((s) => slotKey(s.weekday, s.block)),
    activities: Object.fromEntries(p.activities.map((a) => [a.activityId, a.interest])),
    groupSize: p.groupSize,
    energy: p.energy,
    budget: p.budget,
    // A newer code of conduct must be accepted again.
    conductAccepted: p.conductVersion >= currentConductVersion,
  };
}

/** The SaveProfile payload. conductVersion is the version the person just accepted. */
export function toProfile(d: Draft, conductVersion: number) {
  return {
    displayName: d.displayName.trim(),
    birthYear: Number(d.birthYear),
    gender: d.gender,
    sameGenderOnly: d.sameGenderOnly,
    homeAreaId: d.homeAreaId,
    maxTravelMin: d.maxTravelMin,
    spokenLanguages: d.spoken,
    primaryLanguage: d.primaryLanguage,
    secondaryLanguage: d.secondaryLanguage,
    availability: d.availability.map((k) => {
      const [weekday, block] = k.split(':').map(Number);
      return { weekday, block: block as TimeBlock };
    }),
    activities: Object.entries(d.activities).map(([activityId, interest]) => ({ activityId, interest })),
    groupSize: d.groupSize,
    energy: d.energy,
    budget: d.budget,
    conductVersion,
  };
}
