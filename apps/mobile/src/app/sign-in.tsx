import { Image } from 'expo-image';
import { useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, TextInput, View } from 'react-native';

import { ErrorLine } from '@/components/error-line';
import { ScreenFrame } from '@/components/screen-frame';
import { signInErrorText } from '@/features/auth/sign-in-error';
import { ThemedText } from '@/components/themed-text';
import { BodyFont, FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { PrimaryButton } from '@/components/primary-button';
import { useI18n } from '@/i18n/i18n-provider';
import { CODE_LENGTH } from '@/lib/otp';
import { supabase } from '@/lib/supabase';

/**
 * Passwordless sign-in: the user enters an email, receives a 6-digit code and
 * types it here. The same flow creates the account on first use.
 * On success, the auth listener in AuthProvider switches the navigator away
 * from this screen; nothing here navigates explicitly.
 * Shown in the device language (I18nProvider): nobody has chosen one yet.
 */
export default function SignInScreen() {
  const theme = useTheme();
  const { t, languages } = useI18n();
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [busy, setBusy] = useState(false);
  // The AuthError, never its message: signInErrorText words it by code.
  const [error, setError] = useState<{ code?: string } | null>(null);

  const sendCode = async () => {
    setBusy(true);
    setError(null);
    const { error } = await supabase.auth.signInWithOtp({
      email: email.trim(),
      // data becomes the new account's user_metadata (ignored for existing
      // accounts): the code emails are written in metadata.lang
      // (supabase/templates), so a new user's first email is in the language
      // this screen is showing.
      options: { shouldCreateUser: true, data: { lang: languages.primary } },
    });
    setBusy(false);
    if (error) return setError(error);
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
      setError(error);
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
    // The frame keeps the keyboard off the code field (SDK 58 Android apps are edge-to-edge,
    // so the window no longer resizes for it) and gives the form column.
    <ScreenFrame>
      <View style={styles.content}>
        <Image source={require('@/assets/images/logo.png')} style={styles.logo} contentFit="contain" accessibilityLabel="Issho" />

        {step === 'email' ? (
          <View style={styles.form}>
            <ThemedText type="subtitle">{t('signIn.title')}</ThemedText>
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
            <PrimaryButton label={t('signIn.sendCode')} onPress={sendCode} disabled={!validEmail} busy={busy} />
          </View>
        ) : (
          <View style={styles.form}>
            <ThemedText type="subtitle">{t('signIn.checkEmail')}</ThemedText>
            <ThemedText themeColor="textSecondary">{t('signIn.codeSent', { email: email.trim() })}</ThemedText>
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
              accessibilityLabel={t('signIn.codeLabel')}
            />
            {busy && <ActivityIndicator />}
            <View style={styles.row}>
              <Pressable onPress={sendCode} disabled={busy}>
                <ThemedText type="link">{t('signIn.resend')}</ThemedText>
              </Pressable>
              <Pressable onPress={() => setStep('email')} disabled={busy}>
                <ThemedText type="link">{t('signIn.differentEmail')}</ThemedText>
              </Pressable>
            </View>
          </View>
        )}

        <ErrorLine error={error} text={(e, tr) => signInErrorText(e as { code?: string }, tr)} style={styles.error} />
      </View>
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  content: { flex: 1, justifyContent: 'center', padding: Spacing.four, gap: Spacing.four },
  // Capped so the logo stays a logo on wide screens instead of filling them.
  logo: { width: '70%', maxWidth: 280, aspectRatio: 900 / 598, alignSelf: 'center' },
  form: { gap: Spacing.three },
  input: { fontFamily: BodyFont.regular, borderRadius: 12, paddingHorizontal: Spacing.three, paddingVertical: Spacing.three, fontSize: FontSize.default },
  // The six digits are the one thing on this screen, set large and spaced like a code, so they have a display size of their own.
  // eslint-disable-next-line no-restricted-syntax
  code: { fontSize: 28, letterSpacing: 12, textAlign: 'center' },
  row: { flexDirection: 'row', justifyContent: 'space-between' },
  error: { textAlign: 'center' },
});
