import type { Plan } from '@issho/api-client';

/** A plan as the API returns it (docs: proto/issho/v1/plan.proto), for screen tests. */
export function aPlan(over: Partial<Plan> = {}): Plan {
  return {
    $typeName: 'issho.v1.Plan',
    id: 'p1',
    city: 'da-nang-hoi-an',
    language: 'en',
    title: 'Lantern making in Hoi An',
    description: 'Fold and paint a silk lantern with a local maker.',
    price: 350000,
    currency: 'VND',
    meetingAddress: '12 Nguyen Thai Hoc, Hoi An',
    meetingPoint: 'The blue door beside the tailor',
    whatToBring: ['Sunscreen', 'A small bag'],
    partsOfDay: ['morning'],
    startsAt: '09:00',
    endsAt: '12:30',
    timeZone: 'Asia/Ho_Chi_Minh',
    stops: [
      { $typeName: 'issho.v1.Stop', position: 1, stayMinutes: 90, name: 'Lantern studio', note: 'Bring your own ideas.', leg: { $typeName: 'issho.v1.Leg', mode: 'walk', minutes: 10 } },
      { $typeName: 'issho.v1.Stop', position: 2, stayMinutes: 60, name: 'Riverside café', note: 'Iced coffee to finish.' },
    ],
    signedUp: 0,
    roundEnds: '2026-11-28',
    ...over,
  } as Plan;
}
