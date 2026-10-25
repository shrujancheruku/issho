import { Alert } from 'react-native';

// Confirmations and notices. Alert.alert does nothing on the web, so
// dialogs.web.ts uses the browser's own dialogs instead.

/** Asks before something that can't be undone; onConfirm runs on yes. */
export function confirm(o: { title: string; body: string; confirm: string; cancel: string; onConfirm: () => void }) {
  Alert.alert(o.title, o.body, [
    { text: o.cancel, style: 'cancel' },
    { text: o.confirm, style: 'destructive', onPress: o.onConfirm },
  ]);
}

/** Tells the person something went wrong. */
export function notify(title: string, body: string) {
  Alert.alert(title, body);
}
