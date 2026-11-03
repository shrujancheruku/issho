import type { ReactNode } from 'react';
import { ScrollView, StyleSheet, TextInput, View } from 'react-native';

import { Chip } from '@/components/chip';
import { ScreenFrame, useLeave } from '@/components/screen-frame';
import { ThemedText } from '@/components/themed-text';
import { BodyFont, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/**
 * A short form on its own screen (reporting someone, leaving a crew): a
 * back link, a title with its hint (and anything under it, intro), the
 * form, and a footer kept above the keyboard for its button.
 */
export function FormScreen({
  title,
  hint,
  intro,
  children,
  footer,
}: {
  title: string;
  hint: string;
  intro?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
}) {
  const back = useLeave('/');
  return (
    <ScreenFrame onBack={back}>
      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.titleBlock}>
          <ThemedText type="subtitle">{title}</ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {hint}
          </ThemedText>
          {intro}
        </View>
        {children}
      </ScrollView>
      {footer && <View style={styles.footer}>{footer}</View>}
    </ScreenFrame>
  );
}

/** A labelled single choice shown as chips. */
export function ChipChoice<T extends string | number>({
  label,
  options,
  value,
  onChange,
  children,
}: {
  label: string;
  options: [T, string][];
  value: T;
  onChange: (value: T) => void;
  /** Shown under the chips (e.g. advice for the choice made). */
  children?: ReactNode;
}) {
  return (
    <View style={styles.section}>
      <ThemedText type="smallBold">{label}</ThemedText>
      <View style={styles.wrap}>
        {options.map(([v, text]) => (
          <Chip key={v} label={text} selected={value === v} onPress={() => onChange(v)} />
        ))}
      </View>
      {children}
    </View>
  );
}

/** Free text for a form, up to the server's limit. */
/** `maxLength` is the request's `details` limit: `maxLen(<Request>Schema, 'details')`. */
export function DetailsInput({
  label,
  value,
  onChange,
  maxLength,
}: {
  label: string;
  value: string;
  onChange: (text: string) => void;
  maxLength: number;
}) {
  const theme = useTheme();
  return (
    <View style={styles.section}>
      <ThemedText type="smallBold">{label}</ThemedText>
      <TextInput
        value={value}
        onChangeText={onChange}
        maxLength={maxLength}
        multiline
        style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  content: { padding: Spacing.four, gap: Spacing.four },
  titleBlock: { gap: Spacing.two },
  section: { gap: Spacing.two },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  input: { fontFamily: BodyFont.regular, minHeight: 96, borderRadius: 12, padding: Spacing.three, textAlignVertical: 'top' },
  footer: { padding: Spacing.four, gap: Spacing.two },
});
