import { ConnectError } from "@connectrpc/connect";
import { ErrorReason, ErrorReasonDetailSchema } from "./gen/issho/v1/errors_pb";

/**
 * The machine-readable reason the API attached to a failed call, if any
 * (`ErrorReason.SUSPENDED`, `NO_PROFILE`, ...). Accepts anything thrown: a
 * non-Connect error, or a Connect error without the detail, gives undefined.
 * Branch on the Connect code first; the reason refines it.
 */
export function errorReason(err: unknown): ErrorReason | undefined {
  const reason = ConnectError.from(err).findDetails(ErrorReasonDetailSchema)[0]?.reason;
  return reason === undefined || reason === ErrorReason.UNSPECIFIED ? undefined : reason;
}
