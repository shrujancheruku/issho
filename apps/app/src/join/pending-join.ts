/**
 * The join link the person arrived by, held in memory from the moment the link
 * opens until it has been used. It is the one place the code lives: it is not
 * in the sign-in address's return path, not in storage and not logged, so a
 * redeemed code is gone once the screen has said what happened. A link that
 * opens while signed out waits here through the email-code sign-in.
 */
let pending: string | null = null;

export const setPendingJoin = (code: string) => {
  pending = code;
};
export const peekPendingJoin = () => pending;
export const clearPendingJoin = () => {
  pending = null;
};
