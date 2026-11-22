import { Pressable, StyleSheet, Switch, Text, TextInput, View, type TextInputProps } from 'react-native';

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

/** One answer among several (`radio`) or one of many ticked (`checkbox`). `label` is the translated text. */
export function Choice({ label, selected, onPress, kind }: { label: string; selected: boolean; onPress: () => void; kind: 'radio' | 'checkbox' }) {
  const theme = useTheme();
  return (
    <Pressable
      accessibilityRole={kind}
      accessibilityLabel={label}
      accessibilityState={kind === 'radio' ? { selected } : { checked: selected }}
      onPress={onPress}
      style={[
        styles.choice,
        { borderColor: selected ? theme.accent : theme.lineStrong, backgroundColor: selected ? theme.accent : theme.backgroundElement },
      ]}
    >
      <Text style={[styles.buttonText, { color: selected ? theme.onAccent : theme.text }]}>
        {/* A tick, so a ticked box does not rest on colour alone. The label read aloud is the accessibilityLabel. */}
        {kind === 'checkbox' && selected ? `✓ ${label}` : label}
      </Text>
    </Pressable>
  );
}

/** An on-off setting with its label and a line of help. */
export function Toggle({ label, help, value, onValueChange }: { label: string; help?: string; value: boolean; onValueChange: (value: boolean) => void }) {
  const theme = useTheme();
  return (
    <View style={styles.toggle}>
      <View style={styles.toggleText}>
        <Text style={[styles.buttonText, { color: theme.text }]}>{label}</Text>
        {help ? <Text style={[styles.label, { color: theme.textSecondary }]}>{help}</Text> : null}
      </View>
      <Switch accessibilityLabel={label} value={value} onValueChange={onValueChange} trackColor={{ true: theme.accent }} />
    </View>
  );
}

const styles = StyleSheet.create({
  choice: {
    minHeight: 48,
    borderRadius: 8,
    borderWidth: 1,
    justifyContent: 'center',
    paddingHorizontal: Spacing.three,
  },
  toggle: { minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: Spacing.three },
  toggleText: { flex: 1, gap: Spacing.half },
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
