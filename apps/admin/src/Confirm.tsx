import { useEffect, useRef, useState } from 'react';

/**
 * A button that asks before it acts: the first click only shows "Yes, …" and
 * Cancel; only the Yes button calls onConfirm.
 *
 * By default it folds back to the first button as soon as Yes is clicked, so
 * the parent disables that button while the RPC is in flight and a double
 * click sends one RPC. With keepOpen (the suspend button) the confirmation
 * stays up until the parent removes it, so an error leaves Yes and Cancel to
 * retry or back out.
 *
 * Focus follows the swap: opening the confirmation focuses Yes, and leaving
 * it (Cancel, or Yes with the default fold) returns focus to the first button.
 * Otherwise the unmounted button would drop focus to the page body.
 */
export function ConfirmButton({ label, confirmLabel, cancelLabel = 'Cancel', onConfirm, disabled, className, keepOpen }: {
  label: string; confirmLabel: string; cancelLabel?: string; onConfirm: () => void; disabled?: boolean; className?: string; keepOpen?: boolean;
}) {
  const [confirming, setConfirming] = useState(false);
  const first = useRef<HTMLButtonElement>(null);
  const yes = useRef<HTMLButtonElement>(null);
  const moveFocus = useRef(false); // not on the first render, only after the person acts
  useEffect(() => {
    if (moveFocus.current) (confirming ? yes : first).current?.focus();
  }, [confirming]);
  const set = (next: boolean) => { moveFocus.current = true; setConfirming(next); };

  if (!confirming) return <button ref={first} type="button" className={className} disabled={disabled} onClick={() => set(true)}>{label}</button>;
  return (
    <>
      <button ref={yes} type="button" className={className} disabled={disabled} onClick={() => { if (!keepOpen) set(false); onConfirm(); }}>{confirmLabel}</button>
      <button type="button" className="secondary" onClick={() => set(false)}>{cancelLabel}</button>
    </>
  );
}
