import { CrewResponse, CrewStatus, type Crew } from '@issho/api-client';

/** What an API that doesn't send `Crew.quorum` yet (0) means: matching.CrewMinSize. */
const FALLBACK_QUORUM = 3;

/**
 * A crew as the person sees it: the one place the app's crew rules live
 * (the counterpart of outing() for events), so the crew screen, Together
 * and You only read it. Pure, so the rules are unit-tested.
 */
export type CrewView = {
  /** Still an offer: waiting for answers. */
  isOffer: boolean;
  /** Formed; its chat is open. */
  isActive: boolean;
  /** The person said yes. */
  accepted: boolean;
  /** An offer waiting for the person's answer (Together's "needs you"). */
  needsAnswer: boolean;
  /** Listed as a group chat in Together. */
  listedAsGroup: boolean;
  /** Acceptances that start the crew. */
  quorum: number;
  /** Acceptances so far. */
  yes: number;
  /** `yes`, capped at the quorum, for the counter. */
  yesShown: number;
};

/** No crew rule depends on the time yet, so unlike outing() it takes no `now`. */
export function crewView(crew: Crew): CrewView {
  const isOffer = crew.status === CrewStatus.OFFERED;
  const isActive = crew.status === CrewStatus.ACTIVE;
  const accepted = crew.response === CrewResponse.ACCEPTED;
  const quorum = crew.quorum > 0 ? crew.quorum : FALLBACK_QUORUM;
  const yes = crew.members.filter((m) => m.accepted).length;
  return {
    isOffer,
    isActive,
    accepted,
    needsAnswer: isOffer && !accepted,
    listedAsGroup: isActive,
    quorum,
    yes,
    yesShown: Math.min(yes, quorum),
  };
}
