// Web version of dialogs.ts: Alert.alert is a no-op in browsers, so the
// confirmation never appeared and account deletion couldn't be reached.
// The browser's dialogs are plain but accessible and can't be missed.

export function confirm(o: { title: string; body: string; confirm: string; cancel: string; onConfirm: () => void }) {
  // window.confirm has no custom button labels: the question carries them.
  if (window.confirm(`${o.title}\n\n${o.body}`)) o.onConfirm();
}

export function notify(title: string, body: string) {
  window.alert(`${title}\n\n${body}`);
}
