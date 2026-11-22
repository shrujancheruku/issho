import { SignupProblemReason } from '@issho/api-client';
import { fireEvent, render, screen } from '@testing-library/react-native';
import { Button } from '@/components/form';
import { SignUpRefusal, useSignUp } from '@/join/sign-up';
import { fakeApi } from '@/test-support/fake-api';
import { signupRefusal } from '@/test-support/problems';
import { TestProviders } from '@/test-support/providers';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({
  __esModule: true,
  default: jest.fn(() => 'light'),
}));

// What #616's signup screen does with the entry point: ask to sign up, and show the refusal it gets back.
function Harness() {
  const { signUp, refusal } = useSignUp();
  return (
    <>
      <Button label="Sign up" onPress={() => void signUp('p1', [{ date: '2026-12-01', part: 'evening' }])} />
      <SignUpRefusal refusal={refusal} />
    </>
  );
}

const ticks = [{ date: '2026-12-01', part: 'evening' }];

describe('a signed-in person who is not admitted', () => {
  it('is told they need a join link when they try to sign up', async () => {
    const api = fakeApi();
    api.signup.signUp.mockRejectedValue(signupRefusal(SignupProblemReason.NOT_ADMITTED));
    await render(
      <TestProviders lang="en" api={api}>
        <Harness />
      </TestProviders>,
    );
    expect(screen.queryByRole('alert')).toBeNull();
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('You need a join link to sign up. Ask a member to send you one.')).toBeTruthy();
    expect(api.signup.signUp).toHaveBeenCalledWith({ planId: 'p1', ticks });
  });

  it('is told so in Vietnamese', async () => {
    const api = fakeApi();
    api.signup.signUp.mockRejectedValue(signupRefusal(SignupProblemReason.NOT_ADMITTED));
    await render(
      <TestProviders lang="vi" api={api}>
        <Harness />
      </TestProviders>,
    );
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('Bạn cần một liên kết tham gia để đăng ký. Hãy nhờ một thành viên gửi cho bạn.')).toBeTruthy();
  });

  it('is not told that for any other failure', async () => {
    const api = fakeApi();
    api.signup.signUp.mockRejectedValue(new Error('network'));
    await render(
      <TestProviders lang="en" api={api}>
        <Harness />
      </TestProviders>,
    );
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('We could not sign you up. Try again.')).toBeTruthy();
    expect(screen.queryByText(/join link/)).toBeNull();
  });

  it('sees nothing wrong when the signup works', async () => {
    const api = fakeApi();
    api.signup.signUp.mockResolvedValue({ signup: {} });
    await render(
      <TestProviders lang="en" api={api}>
        <Harness />
      </TestProviders>,
    );
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    await screen.findByRole('button', { name: 'Sign up' });
    expect(screen.queryByRole('alert')).toBeNull();
  });
});
