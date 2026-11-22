import { Pressable, StyleSheet, Text, TextInput, View, type TextInputProps } from 'react-native';

import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/** A labelled text box. `label` is the already-translated text, read aloud and shown above the box. */
export function Field({ label, ...input }: { label: string } & TextInputProps) {
  const theme = useTheme();
  return (
    <View style={styles.field}>
      <Text style={[styles.label, { color: theme.textSecondary }]}>{label}</Text>
      <TextInput
        accessibilityLabel={label}
        placeholderTextColor={theme.textSecondary}
        style={[
          styles.input,
          {
            color: theme.text,
            backgroundColor: theme.backgroundElement,
            borderColor: theme.lineStrong,
          },
        ]}
        {...input}
      />
    </View>
  );
}

/** The main button, or a quiet one (`quiet`) for the secondary action. */
export function Button({ label, onPress, quiet, disabled }: { label: string; onPress: () => void; quiet?: boolean; disabled?: boolean }) {
  const theme = useTheme();
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: !!disabled }}
      disabled={disabled}
      onPress={onPress}
      style={[styles.button, quiet ? null : { backgroundColor: theme.accent }, disabled && styles.disabled]}
    >
      <Text style={[styles.buttonText, { color: quiet ? theme.link : theme.onAccent }]}>{label}</Text>
    </Pressable>
  );
}

/** A message the person needs to read, in the error colour; announced by screen readers when it appears. */
export function ErrorText({ children }: { children: string }) {
  const theme = useTheme();
  return (
    <Text accessibilityRole="alert" style={[styles.error, { color: theme.error }]}>
      {children}
    </Text>
  );
}

const styles = StyleSheet.create({
  field: { gap: Spacing.one },
  label: { fontSize: FontSize.small },
  input: {
    fontSize: FontSize.default,
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: Spacing.three,
    paddingVertical: Spacing.two,
    minHeight: 48,
  },
  button: {
    minHeight: 48,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: Spacing.three,
  },
  buttonText: { fontSize: FontSize.default, fontWeight: '600' },
  disabled: { opacity: 0.5 },
  error: { fontSize: FontSize.small },
});
