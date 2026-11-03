// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';

import SignInScreen from '@/app/sign-in';
import { supabase } from '@/lib/supabase';

// The real screen runs. Supabase Auth's OTP calls are faked (a test never
// reaches a hosted project), and the i18n provider answers with the key so a
// reworded string does not break the flow.
jest.mock('@/lib/supabase', () => ({
  supabase: { auth: { signInWithOtp: jest.fn(), verifyOtp: jest.fn() } },
}));
jest.mock('@/i18n/i18n-provider', () => {
  const i18n = {
    t: (key: string, params?: Record<string, string>) => (params ? `${key} ${Object.values(params).join(' ')}` : key),
    languages: { primary: 'ja', secondary: '' },
  };
  return { useI18n: () => i18n, useOptionalI18n: () => i18n };
});
jest.mock('@/components/screen-frame', () => ({ ScreenFrame: ({ children }: { children: unknown }) => children }));
jest.mock('expo-image', () => ({ Image: () => null }));
// Styling and native chrome are not under test.
jest.mock('@/constants/theme', () => ({ BodyFont: { regular: 'x' }, Spacing: new Proxy({}, { get: () => 4 }) }));
jest.mock('@/hooks/use-theme', () => ({ useTheme: () => new Proxy({}, { get: () => '#000' }) }));
jest.mock('@/components/themed-text', () => {
  const { Text } = jest.requireActual('react-native');
  return { ThemedText: ({ children }: { children: unknown }) => <Text>{children}</Text> };
});
jest.mock('@/components/primary-button', () => {
  const { Pressable, Text } = jest.requireActual('react-native');
  return {
    PrimaryButton: ({ label, onPress, disabled }: { label: string; onPress: () => void; disabled?: boolean }) => (
      <Pressable onPress={onPress} disabled={disabled}>
        <Text>{label}</Text>
      </Pressable>
    ),
  };
});

const auth = (supabase as unknown as { auth: { signInWithOtp: jest.Mock; verifyOtp: jest.Mock } }).auth;

beforeEach(() => {
  auth.signInWithOtp.mockReset().mockResolvedValue({ error: null });
  auth.verifyOtp.mockReset().mockResolvedValue({ error: null });
});

const typeEmail = (text: string) => fireEvent.changeText(screen.getByPlaceholderText('you@example.com'), text);
const typeCode = (text: string) => fireEvent.changeText(screen.getByLabelText('signIn.codeLabel'), text);

/** Through the email step to the code step. */
async function toCodeStep() {
  await render(<SignInScreen />);
  await typeEmail('  mika@example.com ');
  await fireEvent.press(screen.getByText('signIn.sendCode'));
  await screen.findByText('signIn.checkEmail');
}

describe('step 1: the email', () => {
  it('will not send an address that is not one', async () => {
    await render(<SignInScreen />);
    await typeEmail('mika@');
    await fireEvent.press(screen.getByText('signIn.sendCode'));
    expect(auth.signInWithOtp).not.toHaveBeenCalled();
    expect(screen.queryByText('signIn.checkEmail')).toBeNull();
  });

  it('sends a code to the trimmed address, in the language on screen, and moves to the code step', async () => {
    await toCodeStep();
    expect(auth.signInWithOtp).toHaveBeenCalledWith({
      email: 'mika@example.com',
      options: { shouldCreateUser: true, data: { lang: 'ja' } },
    });
    expect(screen.getByText('signIn.codeSent mika@example.com')).toBeTruthy();
  });

  it('stays on the email step and words the error by its code when sending fails', async () => {
    auth.signInWithOtp.mockResolvedValue({ error: { code: 'over_email_send_rate_limit', message: 'English for developers' } });
    await render(<SignInScreen />);
    await typeEmail('mika@example.com');
    await fireEvent.press(screen.getByText('signIn.sendCode'));
    expect(await screen.findByText('signIn.errorRateLimit')).toBeTruthy();
    expect(screen.queryByText('English for developers')).toBeNull();
    expect(screen.queryByText('signIn.checkEmail')).toBeNull();
  });
});

describe('step 2: the code', () => {
  it('signs in as soon as the sixth digit is in, with the digits only, and navigates nowhere itself', async () => {
    await toCodeStep();
    await typeCode('123');
    expect(auth.verifyOtp).not.toHaveBeenCalled();
    await typeCode('123 456');
    await waitFor(() => expect(auth.verifyOtp).toHaveBeenCalledTimes(1));
    expect(auth.verifyOtp).toHaveBeenCalledWith({ email: 'mika@example.com', token: '123456', type: 'email' });
    expect(screen.queryByText('signIn.errorExpired')).toBeNull();
  });

  it('ignores letters and anything past six digits', async () => {
    await toCodeStep();
    await typeCode('12a34b567890');
    await waitFor(() => expect(auth.verifyOtp).toHaveBeenCalledTimes(1));
    expect(auth.verifyOtp).toHaveBeenCalledWith({ email: 'mika@example.com', token: '123456', type: 'email' });
  });

  it('a wrong code clears the box, says so and lets the person try again', async () => {
    auth.verifyOtp.mockResolvedValueOnce({ error: { code: 'otp_expired', message: 'Token has expired or is invalid' } });
    await toCodeStep();
    await typeCode('000000');
    expect(await screen.findByText('signIn.errorExpired')).toBeTruthy();
    expect(screen.getByLabelText('signIn.codeLabel').props.value).toBe('');

    await typeCode('123456');
    await waitFor(() => expect(auth.verifyOtp).toHaveBeenCalledTimes(2));
    expect(auth.verifyOtp).toHaveBeenLastCalledWith({ email: 'mika@example.com', token: '123456', type: 'email' });
    await waitFor(() => expect(screen.queryByText('signIn.errorExpired')).toBeNull());
  });

  it('resend sends another code to the same address and clears the box', async () => {
    await toCodeStep();
    await typeCode('12');
    await fireEvent.press(screen.getByText('signIn.resend'));
    await waitFor(() => expect(auth.signInWithOtp).toHaveBeenCalledTimes(2));
    expect(auth.signInWithOtp).toHaveBeenLastCalledWith({
      email: 'mika@example.com',
      options: { shouldCreateUser: true, data: { lang: 'ja' } },
    });
    await waitFor(() => expect(screen.getByLabelText('signIn.codeLabel').props.value).toBe(''));
    expect(screen.getByText('signIn.checkEmail')).toBeTruthy();
  });

  it('a failed resend shows the error and keeps the code step', async () => {
    await toCodeStep();
    auth.signInWithOtp.mockResolvedValueOnce({ error: { code: 'over_email_send_rate_limit' } });
    await fireEvent.press(screen.getByText('signIn.resend'));
    expect(await screen.findByText('signIn.errorRateLimit')).toBeTruthy();
    expect(screen.getByText('signIn.checkEmail')).toBeTruthy();
  });

  it('"use a different email" goes back to the email step, keeping what was typed', async () => {
    await toCodeStep();
    await fireEvent.press(screen.getByText('signIn.differentEmail'));
    expect(screen.getByText('signIn.title')).toBeTruthy();
    expect(screen.getByPlaceholderText('you@example.com').props.value).toBe('  mika@example.com ');
  });

  it('an error nobody worded reads as the generic one', async () => {
    auth.verifyOtp.mockResolvedValueOnce({ error: { message: 'boom' } });
    await toCodeStep();
    await typeCode('111111');
    expect(await screen.findByText('error.generic')).toBeTruthy();
  });
});
