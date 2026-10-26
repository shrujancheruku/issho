import { router } from 'expo-router';
import type { ReactNode } from 'react';
import { KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Chip } from '@/components/chip';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BodyFont, FormWidth, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/** The longest details text, as the server (plans.MaxReportDetails). */
const MaxDetails = 1000;

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
  const { t } = useI18n();
  const back = () => (router.canGoBack() ? router.back() : router.replace('/'));
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={back} hitSlop={8}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
          </View>
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
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
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
export function DetailsInput({ label, value, onChange }: { label: string; value: string; onChange: (text: string) => void }) {
  const theme = useTheme();
  return (
    <View style={styles.section}>
      <ThemedText type="smallBold">{label}</ThemedText>
      <TextInput
        value={value}
        onChangeText={onChange}
        maxLength={MaxDetails}
        multiline
        style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
      />
    </View>
  );
}

/** A form footer's error line. */
export function FormError({ children }: { children: string }) {
  return (
    <ThemedText themeColor="error" style={styles.error}>
      {children}
    </ThemedText>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingTop: Spacing.three },
  content: { padding: Spacing.four, gap: Spacing.four },
  titleBlock: { gap: Spacing.two },
  section: { gap: Spacing.two },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  input: { fontFamily: BodyFont.regular, minHeight: 96, borderRadius: 12, padding: Spacing.three, textAlignVertical: 'top' },
  footer: { padding: Spacing.four, gap: Spacing.two },
  error: { textAlign: 'center' },
});
