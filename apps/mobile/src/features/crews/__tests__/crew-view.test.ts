/// <reference types="jest" />
import { CrewResponse, CrewStatus, type Crew } from '@issho/api-client';

import { crewView } from '../crew-view';

const crew = (o: { status: CrewStatus; response?: CrewResponse; quorum?: number; yes?: number }) =>
  ({
    id: 'c',
    status: o.status,
    response: o.response ?? CrewResponse.OFFERED,
    quorum: o.quorum ?? 0,
    members: Array.from({ length: 4 }, (_, i) => ({ id: `m${i}`, accepted: i < (o.yes ?? 0) })),
  }) as unknown as Crew;

describe('crewView', () => {
  it('an offer the person has not answered needs them', () => {
    const v = crewView(crew({ status: CrewStatus.OFFERED }));
    expect(v).toMatchObject({ isOffer: true, isActive: false, accepted: false, needsAnswer: true, listedAsGroup: false });
  });

  it('an offer the person accepted waits without needing them', () => {
    const v = crewView(crew({ status: CrewStatus.OFFERED, response: CrewResponse.ACCEPTED }));
    expect(v).toMatchObject({ isOffer: true, accepted: true, needsAnswer: false, listedAsGroup: false });
  });

  it('an active crew is a group, not an answer', () => {
    const v = crewView(crew({ status: CrewStatus.ACTIVE, response: CrewResponse.ACCEPTED }));
    expect(v).toMatchObject({ isOffer: false, isActive: true, needsAnswer: false, listedAsGroup: true });
  });

  it('lapsed and closed crews are neither', () => {
    for (const status of [CrewStatus.LAPSED, CrewStatus.CLOSED]) {
      expect(crewView(crew({ status }))).toMatchObject({ isOffer: false, isActive: false, needsAnswer: false, listedAsGroup: false });
    }
  });

  it('takes the quorum from the field', () => {
    const v = crewView(crew({ status: CrewStatus.OFFERED, quorum: 4, yes: 4 }));
    expect(v).toMatchObject({ quorum: 4, yes: 4, yesShown: 4 });
  });

  it('falls back to 3 from an older API, and caps the count', () => {
    const v = crewView(crew({ status: CrewStatus.OFFERED, quorum: 0, yes: 4 }));
    expect(v).toMatchObject({ quorum: 3, yes: 4, yesShown: 3 });
  });
});
