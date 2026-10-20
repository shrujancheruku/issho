import { Image } from 'expo-image';
import { useState } from 'react';
import { ActivityIndicator, KeyboardAvoidingView, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { supabase } from '@/lib/supabase';

const CODE_LENGTH = 6; // matches [auth.email] otp_length in supabase/config.toml

/**
 * Passwordless sign-in: the user enters an email, receives a 6-digit code and
 * types it here. The same flow creates the account on first use.
 * On success, the auth listener in AuthProvider switches the navigator away
 * from this screen; nothing here navigates explicitly.
 */
export default function SignInScreen() {
  const theme = useTheme();
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sendCode = async () => {
    setBusy(true);
    setError(null);
    const { error } = await supabase.auth.signInWithOtp({
      email: email.trim(),
      options: { shouldCreateUser: true },
    });
    setBusy(false);
    if (error) return setError(error.message);
    setCode('');
    setStep('code');
  };

  const verify = async (token: string) => {
    setBusy(true);
    setError(null);
    const { error } = await supabase.auth.verifyOtp({ email: email.trim(), token, type: 'email' });
    setBusy(false);
    if (error) {
      setCode('');
      setError(error.message);
    }
  };

  const onCodeChange = (text: string) => {
    const digits = text.replace(/\D/g, '').slice(0, CODE_LENGTH);
    setCode(digits);
    // Submit as soon as the code is complete, including when iOS autofills it from Mail.
    if (digits.length === CODE_LENGTH) void verify(digits);
  };

  const inputStyle = [styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }];
  const validEmail = /^\S+@\S+\.\S+$/.test(email.trim());

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        {/* Padding on both platforms: SDK 58 Android apps are edge-to-edge, so the window no
            longer resizes for the keyboard and the code field would sit behind it. */}
        <KeyboardAvoidingView behavior="padding" style={styles.content}>
          <Image source={require('@/assets/images/logo.png')} style={styles.logo} contentFit="contain" accessibilityLabel="Issho" />

          {step === 'email' ? (
            <View style={styles.form}>
              <ThemedText type="subtitle">Sign in or create an account</ThemedText>
              <TextInput
                style={inputStyle}
                value={email}
                onChangeText={setEmail}
                placeholder="you@example.com"
                placeholderTextColor={theme.textSecondary}
                keyboardType="email-address"
                autoCapitalize="none"
                autoCorrect={false}
                autoComplete="email"
                textContentType="emailAddress"
                returnKeyType="send"
                onSubmitEditing={() => validEmail && void sendCode()}
              />
              <PrimaryButton label="Send code" onPress={sendCode} disabled={!validEmail} busy={busy} />
            </View>
          ) : (
            <View style={styles.form}>
              <ThemedText type="subtitle">Check your email</ThemedText>
              <ThemedText themeColor="textSecondary">We sent a {CODE_LENGTH}-digit code to {email.trim()}.</ThemedText>
              <TextInput
                style={[inputStyle, styles.code]}
                value={code}
                onChangeText={onCodeChange}
                keyboardType="number-pad"
                autoComplete="one-time-code"
                textContentType="oneTimeCode"
                maxLength={CODE_LENGTH}
                autoFocus
                editable={!busy}
                accessibilityLabel="Sign-in code"
              />
              {busy && <ActivityIndicator />}
              <View style={styles.row}>
                <Pressable onPress={sendCode} disabled={busy}>
                  <ThemedText type="link">Resend code</ThemedText>
                </Pressable>
                <Pressable onPress={() => setStep('email')} disabled={busy}>
                  <ThemedText type="link">Use a different email</ThemedText>
                </Pressable>
              </View>
            </View>
          )}

          {error && <ThemedText style={styles.error}>{error}</ThemedText>}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

function PrimaryButton({ label, onPress, disabled, busy }: { label: string; onPress: () => void; disabled?: boolean; busy?: boolean }) {
  return (
    <Pressable
      accessibilityRole="button"
      onPress={onPress}
      disabled={disabled || busy}
      style={({ pressed }) => [styles.button, (disabled || busy) && styles.buttonDisabled, pressed && styles.pressed]}>
      {busy ? <ActivityIndicator color="#ffffff" /> : <ThemedText style={styles.buttonLabel}>{label}</ThemedText>}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  content: { flex: 1, justifyContent: 'center', padding: Spacing.four, gap: Spacing.four },
  logo: { width: '70%', aspectRatio: 900 / 598, alignSelf: 'center' },
  form: { gap: Spacing.three },
  input: { borderRadius: 12, paddingHorizontal: Spacing.three, paddingVertical: Spacing.three, fontSize: 17 },
  code: { fontSize: 28, letterSpacing: 12, textAlign: 'center' },
  row: { flexDirection: 'row', justifyContent: 'space-between' },
  button: { backgroundColor: Brand.red, borderRadius: 12, paddingVertical: Spacing.three, alignItems: 'center' },
  buttonDisabled: { opacity: 0.4 },
  pressed: { opacity: 0.8 },
  buttonLabel: { color: '#ffffff', fontWeight: '600' },
  error: { color: Brand.red, textAlign: 'center' },
});
