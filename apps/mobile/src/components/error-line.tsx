import { StyleSheet, type StyleProp, type TextStyle } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { errorText } from '@/lib/error-text';

type Props = {
  /** What failed (a mutation's or query's `error`); nothing shows while it's empty. */
  error: unknown;
  /** Words for an error in particular; defaults to errorText, which never shows the server's message. */
  text?: (err: unknown, t: (key: StringKey) => string) => string;
  type?: 'small';
  /** Centered, as in a form footer. */
  center?: boolean;
  style?: StyleProp<TextStyle>;
};

/** The one error line: form footers, a button's result, chat. */
export function ErrorLine({ error, text = errorText, type, center, style }: Props) {
  const { t } = useI18n();
  if (error === null || error === undefined) return null;
  return (
    <ThemedText type={type} themeColor="error" style={[center && styles.center, style]}>
      {text(error, t)}
    </ThemedText>
  );
}

const styles = StyleSheet.create({ center: { textAlign: 'center' } });
