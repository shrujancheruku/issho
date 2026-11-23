import { ConnectError } from '@connectrpc/connect';
import { JoinLinkProblemReason, JoinLinkProblemSchema, SignupProblemReason, SignupProblemSchema } from '@issho/api-client';

/** The reason the API refused a join-link call, or undefined for any other failure (a network error, a server error). */
export function joinLinkReason(err: unknown): JoinLinkProblemReason | undefined {
  return ConnectError.from(err).findDetails(JoinLinkProblemSchema)[0]?.reason;
}

/** True when a signup was refused because the caller has not been admitted: they need a join link. */
export function isNotAdmitted(err: unknown): boolean {
  return ConnectError.from(err).findDetails(SignupProblemSchema)[0]?.reason === SignupProblemReason.NOT_ADMITTED;
}
