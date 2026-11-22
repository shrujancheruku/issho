import { ConnectError } from '@connectrpc/connect';

import { ProfileProblemSchema, type ProfileProblemReason } from './gen/issho/v1/profile_pb';

/**
 * The reason an RPC refused the caller about their profile (the `ProfileProblem`
 * detail on the error), or undefined for any other error. The app acts on the
 * reason (open the questions, stop an under-18, say women-only is not offered),
 * so the detail is read in one place.
 */
export function profileProblem(err: unknown): ProfileProblemReason | undefined {
  return ConnectError.from(err).findDetails(ProfileProblemSchema)[0]?.reason;
}
