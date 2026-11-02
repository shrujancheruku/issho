/** A protobuf Timestamp, as far as the app reads it. */
type Timestamp = { seconds: bigint } | undefined;

/** A protobuf Timestamp as epoch milliseconds; NaN when unset, so comparisons with it are false. */
export const ms = (ts: Timestamp) => (ts ? Number(ts.seconds) * 1000 : NaN);
