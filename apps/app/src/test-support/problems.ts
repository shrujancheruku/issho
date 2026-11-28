import { Code, ConnectError } from '@connectrpc/connect';
import { JoinLinkProblemReason, JoinLinkProblemSchema, SignupProblemReason, SignupProblemSchema } from '@issho/api-client';

/**
 * The refusal the API sends: FAILED_PRECONDITION carrying a problem detail,
 * which the app reads with `findDetails(...Schema)`. A test builds it here so a
 * screen test fails the way the real client's error does.
 */
export function joinLinkRefusal(reason: JoinLinkProblemReason) {
  return new ConnectError('refused', Code.FailedPrecondition, undefined, [{ desc: JoinLinkProblemSchema, value: { reason } as never }]);
}

export function signupRefusal(reason: SignupProblemReason) {
  return new ConnectError('refused', Code.FailedPrecondition, undefined, [{ desc: SignupProblemSchema, value: { reason } as never }]);
}
