/** An RPC answer the test releases by hand, to act while the call is still in flight. */
export function deferred() {
  let release!: () => void;
  const promise = new Promise<object>((resolve) => { release = () => resolve({}); });
  return { promise, release };
}
