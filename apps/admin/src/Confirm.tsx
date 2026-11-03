import { useState } from 'react';

/**
 * A button that asks before it acts, like the suspend button in Person.tsx:
 * the first click only shows "Yes, …" and Cancel; only the Yes button calls
 * onConfirm. It folds back to the first button when it is used, so a double
 * click on Yes sends one RPC (the second click lands on the first button).
 */
export function ConfirmButton({ label, confirmLabel, onConfirm, disabled, className }: {
  label: string; confirmLabel: string; onConfirm: () => void; disabled?: boolean; className?: string;
}) {
  const [confirming, setConfirming] = useState(false);
  if (!confirming) return <button type="button" className={className} disabled={disabled} onClick={() => setConfirming(true)}>{label}</button>;
  return (
    <>
      <button type="button" className={className} disabled={disabled} onClick={() => { setConfirming(false); onConfirm(); }}>{confirmLabel}</button>
      <button type="button" className="secondary" onClick={() => setConfirming(false)}>Cancel</button>
    </>
  );
}
