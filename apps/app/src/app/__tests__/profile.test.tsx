import { ConnectError, Code } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import { useRouter } from 'expo-router';
import type { ReactNode } from 'react';
import { Pressable, StyleSheet, Text } from 'react-native';
import { ProfileProblemReason, ProfileProblemSchema } from '@issho/api-client';

import { useProfileGate } from '@/profile/use-profile-gate';
import { fakeApi } from '@/test-support/fake-api';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TabsLayout from '../(tabs)/_layout';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import Index from '../index';
import ProfileScreen from '../profile';

jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({
  __esModule: true,
  default: jest.fn(() => 'light'),
}));

// Stands in for #616's signup screen: the one call it will make is the gate.
function SignupStub() {
  const gate = useProfileGate();
  const router = useRouter();
  return (
    <>
      <Text>Signup screen</Text>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Sign up"
        onPress={async () => {
          if (await gate('/signup-stub')) router.setParams({ ok: '1' });
        }}>
        <Text>Sign up</Text>
      </Pressable>
    </>
  );
}

async function open(path: string, api = fakeApi(), lang: 'en' | 'vi' = 'en') {
  await renderRouter(
    {
      index: Index,
      profile: ProfileScreen,
      'signup-stub': SignupStub,
      '(tabs)/_layout': TabsLayout,
      '(tabs)/together': TogetherScreen,
      '(tabs)/plans': PlansScreen,
      '(tabs)/you': YouScreen,
    },
    {
      initialUrl: path,
      wrapper: ({ children }: { children: ReactNode }) => (
        <TestProviders lang={lang} api={api}>
          {children}
        </TestProviders>
      ),
    } as never,
  );
  return api;
}

const press = async (name: string) => fireEvent.press(screen.getByRole('button', { name }));
const next = () => press('Next');
const problem = (reason: ProfileProblemReason, code = Code.InvalidArgument) =>
  new ConnectError('refused', code, undefined, [{ desc: ProfileProblemSchema, value: create(ProfileProblemSchema, { reason }) }]);

async function toGender(api: ReturnType<typeof fakeApi>) {
  await open('/profile?next=/signup-stub', api);
  await fireEvent.changeText(await screen.findByLabelText('The name you go by'), 'Ada');
  await next();
  await fireEvent.changeText(await screen.findByLabelText('Date of birth (YYYY-MM-DD)'), '1990-04-23');
  await next();
  await screen.findByText('Gender');
}

describe('the profile, asked at the first signup', () => {
  it('opens the questions in order when a signup finds no profile, and returns to the signup when done', async () => {
    const api = await open('/signup-stub');
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));

    // 1. name
    await waitFor(() => expect(screen).toHavePathname('/profile'));
    expect(screen).toHaveSearchParams({ next: '/signup-stub' });
    await fireEvent.changeText(await screen.findByLabelText('The name you go by'), 'Ada');
    await next();
    // 2. date of birth
    await fireEvent.changeText(await screen.findByLabelText('Date of birth (YYYY-MM-DD)'), '1990-04-23');
    await next();
    // 3. gender, with its reason
    expect(await screen.findByText('We ask so that nobody ends up the only woman or the only man in a group.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('radio', { name: 'Woman' }));
    await fireEvent.press(screen.getByRole('switch', { name: 'Only women and non-binary people' }));
    await next();
    // 4. spoken languages
    expect(await screen.findByText('Which languages can you comfortably hold a conversation in for a whole outing?')).toBeTruthy();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'English' }));
    await fireEvent.press(screen.getByRole('checkbox', { name: 'Vietnamese' }));
    await next();
    // 5. where they live
    expect(await screen.findByText('Do you live in Da Nang–Hoi An?')).toBeTruthy();
    await fireEvent.press(screen.getByRole('radio', { name: 'Yes' }));
    await next();
    // 6. the code of conduct
    expect(await screen.findByText('The code of conduct')).toBeTruthy();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'I agree to the code of conduct' }));
    await press('Save');

    await waitFor(() => expect(screen).toHavePathname('/signup-stub'));
    expect(api.profile.saveProfile).toHaveBeenCalledWith({
      name: 'Ada',
      dateOfBirth: '1990-04-23',
      gender: 'woman',
      languages: ['en', 'vi'],
      livesInCity: true,
      acceptedCodeOfConduct: true,
      onlyWomenAndNonBinary: true,
    });
  });

  it('lets a signup go on when the person already has a profile', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada', gender: 'woman', languages: ['en'], livesInCity: true, onlyWomenAndNonBinary: false, codeOfConductVersion: 1 } });
    await open('/signup-stub', api);
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    await waitFor(() => expect(api.profile.getProfile).toHaveBeenCalled());
    await waitFor(() => expect(screen).toHaveSearchParams({ ok: '1' }));
    expect(screen).toHavePathname('/signup-stub');
  });

  it('asks for a name of 1 to 40 characters', async () => {
    await open('/profile');
    const field = await screen.findByLabelText('The name you go by');
    await next();
    expect(await screen.findByText('Use 1 to 40 characters.')).toBeTruthy();
    await fireEvent.changeText(field, 'x'.repeat(41));
    await next();
    expect(await screen.findByText('Use 1 to 40 characters.')).toBeTruthy();
    await fireEvent.changeText(field, 'Ada');
    await next();
    expect(await screen.findByLabelText('Date of birth (YYYY-MM-DD)')).toBeTruthy();
  });

  it('stops a date of birth under 18 with a plain message, and asks for a real date', async () => {
    await open('/profile');
    await fireEvent.changeText(await screen.findByLabelText('The name you go by'), 'Ada');
    await next();
    const dob = await screen.findByLabelText('Date of birth (YYYY-MM-DD)');
    await fireEvent.changeText(dob, 'tomorrow');
    await next();
    expect(await screen.findByText('Enter a real date, like 1995-04-23.')).toBeTruthy();
    const now = new Date();
    const underAge = `${now.getFullYear() - 17}-01-01`;
    await fireEvent.changeText(dob, underAge);
    await next();
    expect(await screen.findByText('You need to be 18 or over to use Issho.')).toBeTruthy();
    expect(screen.queryByText('Gender')).toBeNull();
  });

  it("stops a date of birth the API says is under 18, whatever the app's own clock thinks", async () => {
    const api = fakeApi();
    api.profile.saveProfile.mockRejectedValue(problem(ProfileProblemReason.UNDER_AGE));
    await toGender(api);
    await fireEvent.press(screen.getByRole('radio', { name: 'Man' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'English' }));
    await next();
    await fireEvent.press(screen.getByRole('radio', { name: 'No' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'I agree to the code of conduct' }));
    await press('Save');
    expect(await screen.findByText('You need to be 18 or over to use Issho.')).toBeTruthy();
    expect(screen).toHavePathname('/profile');
  });

  it('requires a gender answer and says why it is asked', async () => {
    await toGender(fakeApi());
    expect(screen.getByText('We ask so that nobody ends up the only woman or the only man in a group.')).toBeTruthy();
    await next();
    expect(await screen.findByText('Choose an answer.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('radio', { name: 'Prefer not to say' }));
    await next();
    expect(await screen.findByText('Which languages can you comfortably hold a conversation in for a whole outing?')).toBeTruthy();
  });

  it.each([
    ['Woman', true],
    ['Non-binary', true],
    ['Man', false],
    ['Prefer not to say', false],
  ])('offers "only women and non-binary people" to %s: %s', async (gender, offered) => {
    await toGender(fakeApi());
    await fireEvent.press(screen.getByRole('radio', { name: gender }));
    expect(!!screen.queryByRole('switch', { name: 'Only women and non-binary people' })).toBe(offered);
  });

  it('turns the women-only setting off when the person picks another gender', async () => {
    const api = fakeApi();
    await toGender(api);
    await fireEvent.press(screen.getByRole('radio', { name: 'Woman' }));
    await fireEvent.press(screen.getByRole('switch', { name: 'Only women and non-binary people' }));
    await fireEvent.press(screen.getByRole('radio', { name: 'Man' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'English' }));
    await next();
    await fireEvent.press(screen.getByRole('radio', { name: 'Yes' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'I agree to the code of conduct' }));
    await press('Save');
    await waitFor(() => expect(api.profile.saveProfile).toHaveBeenCalled());
    expect(api.profile.saveProfile.mock.calls[0][0]).toMatchObject({ gender: 'man', onlyWomenAndNonBinary: false });
  });

  it('needs at least one spoken language, an answer about the city and the code of conduct', async () => {
    await toGender(fakeApi());
    await fireEvent.press(screen.getByRole('radio', { name: 'Man' }));
    await next();
    await next();
    expect(await screen.findByText('Choose at least one.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'Vietnamese' }));
    await next();
    await next();
    expect(await screen.findByText('Choose an answer.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('radio', { name: 'No' }));
    await next();
    await press('Save');
    expect(await screen.findByText('Agree to the code of conduct to go on.')).toBeTruthy();
  });

  it('says when the profile could not be saved, and keeps the answers', async () => {
    const api = fakeApi();
    api.profile.saveProfile.mockRejectedValue(new Error('network'));
    await toGender(api);
    await fireEvent.press(screen.getByRole('radio', { name: 'Man' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'English' }));
    await next();
    await fireEvent.press(screen.getByRole('radio', { name: 'No' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'I agree to the code of conduct' }));
    await press('Save');
    expect(await screen.findByText('We could not save your profile. Try again.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Save' })).toBeTruthy();
  });

  it('goes back a step without losing the answer', async () => {
    await open('/profile');
    await fireEvent.changeText(await screen.findByLabelText('The name you go by'), 'Ada');
    await next();
    await screen.findByLabelText('Date of birth (YYYY-MM-DD)');
    await press('Back');
    expect((await screen.findByLabelText('The name you go by')).props.value).toBe('Ada');
  });

  it('ignores a ?next= that leaves the app', async () => {
    await open('/profile?next=https://evil.example');
    await fireEvent.changeText(await screen.findByLabelText('The name you go by'), 'Ada');
    await next();
    // Walk to the end quickly, with the API stand-in saving.
    await fireEvent.changeText(await screen.findByLabelText('Date of birth (YYYY-MM-DD)'), '1990-04-23');
    await next();
    await fireEvent.press(await screen.findByRole('radio', { name: 'Man' }));
    await next();
    await fireEvent.press(await screen.findByRole('checkbox', { name: 'English' }));
    await next();
    await fireEvent.press(await screen.findByRole('radio', { name: 'No' }));
    await next();
    await fireEvent.press(await screen.findByRole('checkbox', { name: 'I agree to the code of conduct' }));
    await press('Save');
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('starts below the top inset, so the title is not under the status bar', async () => {
    const insets = jest.requireMock('react-native-safe-area-context').useSafeAreaInsets as jest.Mock;
    insets.mockReturnValue({ top: 40, bottom: 0, left: 0, right: 0 });
    await open('/profile');
    await screen.findByLabelText('The name you go by');
    expect(StyleSheet.flatten(screen.getByTestId('screen').props.style).paddingTop).toBeGreaterThanOrEqual(40);
    insets.mockReturnValue({ top: 0, bottom: 0, left: 0, right: 0 });
  });

  // Every kind of control on these screens: text boxes, radios, switches, checkboxes and buttons.
  const targets = () => [
    ...screen.queryAllByRole('radio'),
    ...screen.queryAllByRole('checkbox'),
    ...screen.queryAllByRole('switch'),
    ...screen.queryAllByRole('button'),
  ];
  const expectLabelledAndBig = (controls: ReturnType<typeof targets>) => {
    for (const control of controls) {
      expect(control.props.accessibilityLabel ?? control.props.children).toBeTruthy();
      expect(StyleSheet.flatten(control.props.style).minHeight).toBeGreaterThanOrEqual(44);
    }
  };

  it('gives every control a label and a 44-point target', async () => {
    const api = fakeApi();
    await toGender(api);
    await fireEvent.press(screen.getByRole('radio', { name: 'Woman' }));
    expect(screen.getByRole('switch')).toBeTruthy();
    const kinds = new Set(targets().map((c) => c.props.accessibilityRole));
    expect([...kinds]).toEqual(expect.arrayContaining(['radio', 'switch', 'button']));
    expectLabelledAndBig(targets());

    await next();
    expect(await screen.findByLabelText('Search every language')).toBeTruthy();
    expect(screen.getAllByRole('checkbox').length).toBeGreaterThan(4);
    expectLabelledAndBig(targets());
    await fireEvent.press(screen.getByRole('checkbox', { name: 'English' }));
    await next();
    await fireEvent.press(screen.getByRole('radio', { name: 'Yes' }));
    await next();
    expect(screen.getByRole('checkbox', { name: 'I agree to the code of conduct' })).toBeTruthy();
    expectLabelledAndBig(targets());
  });

  it('gives the name and date of birth boxes a 44-point target too', async () => {
    await open('/profile');
    const name = await screen.findByLabelText('The name you go by');
    expect(StyleSheet.flatten(name.props.style).minHeight).toBeGreaterThanOrEqual(44);
    await fireEvent.changeText(name, 'Ada');
    await next();
    const dob = await screen.findByLabelText('Date of birth (YYYY-MM-DD)');
    expect(StyleSheet.flatten(dob.props.style).minHeight).toBeGreaterThanOrEqual(44);
  });

  it('flips the switch from its whole row, the label included', async () => {
    await toGender(fakeApi());
    await fireEvent.press(screen.getByRole('radio', { name: 'Woman' }));
    const row = screen.getByRole('switch', { name: 'Only women and non-binary people' });
    expect(row.props.accessibilityState).toMatchObject({ checked: false });
    await fireEvent.press(screen.getByText('Only women and non-binary people'));
    expect(screen.getByRole('switch', { name: 'Only women and non-binary people' }).props.accessibilityState).toMatchObject({ checked: true });
  });

  it('lets any language be ticked by searching its name, and lists what was ticked', async () => {
    const api = fakeApi();
    await toGender(api);
    await fireEvent.press(screen.getByRole('radio', { name: 'Man' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'English' }));
    expect(screen.queryByRole('checkbox', { name: 'Filipino' })).toBeNull();
    await fireEvent.changeText(screen.getByLabelText('Search every language'), 'filip');
    await fireEvent.press(await screen.findByRole('checkbox', { name: 'Filipino' }));
    await fireEvent.changeText(screen.getByLabelText('Search every language'), '');
    // Ticked, so still shown with the short list once the search is cleared.
    expect(screen.getByRole('checkbox', { name: 'Filipino' }).props.accessibilityState).toMatchObject({ checked: true });
    await next();
    await fireEvent.press(screen.getByRole('radio', { name: 'No' }));
    await next();
    await fireEvent.press(screen.getByRole('checkbox', { name: 'I agree to the code of conduct' }));
    await press('Save');
    await waitFor(() => expect(api.profile.saveProfile).toHaveBeenCalled());
    expect(api.profile.saveProfile.mock.calls[0][0].languages).toEqual(['en', 'fil']);
  });

  it('lists what the search finds first, so it is on screen beside the box', async () => {
    await toGender(fakeApi());
    await fireEvent.press(screen.getByRole('radio', { name: 'Man' }));
    await next();
    await fireEvent.changeText(screen.getByLabelText('Search every language'), 'hawaii');
    expect(screen.getAllByRole('checkbox')[0].props.accessibilityLabel).toBe('Hawaiian');
    await fireEvent.changeText(screen.getByLabelText('Search every language'), 'zzzz');
    expect(screen.getByText('No language has that name.')).toBeTruthy();
  });

  it('searches the language names in the display language', async () => {
    await open('/profile', fakeApi(), 'vi');
    await fireEvent.changeText(await screen.findByLabelText('Tên bạn muốn được gọi'), 'Ada');
    await fireEvent.press(screen.getByRole('button', { name: 'Tiếp' }));
    await fireEvent.changeText(await screen.findByLabelText('Ngày sinh (NNNN-TT-NN)'), '1990-04-23');
    await fireEvent.press(screen.getByRole('button', { name: 'Tiếp' }));
    await screen.findAllByRole('radio');
    await fireEvent.press(screen.getAllByRole('radio')[1]);
    await fireEvent.press(screen.getByRole('button', { name: 'Tiếp' }));
    await fireEvent.changeText(await screen.findByLabelText('Tìm mọi ngôn ngữ'), 'hawaii');
    expect(await screen.findByRole('checkbox', { name: 'Tiếng Hawaii' })).toBeTruthy();
  });

  it('is asked in Vietnamese too', async () => {
    await open('/profile', fakeApi(), 'vi');
    expect(await screen.findByLabelText('Tên bạn muốn được gọi')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Tiếp' })).toBeTruthy();
  });
});

describe('the profile on the You tab', () => {
  const saved = { name: 'Ada', gender: 'woman', languages: ['en', 'vi'], livesInCity: true, onlyWomenAndNonBinary: true, codeOfConductVersion: 1 };

  it('shows the profile and lets the person change it, without asking for the date of birth again', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: saved });
    await open('/you', api);
    expect(await screen.findByText('Ada')).toBeTruthy();
    expect(screen.getByText('English, Vietnamese')).toBeTruthy();

    await fireEvent.press(screen.getByRole('button', { name: 'Change your profile' }));
    await waitFor(() => expect(screen).toHavePathname('/profile'));
    const name = await screen.findByLabelText('The name you go by');
    expect(name.props.value).toBe('Ada');
    expect(screen.queryByLabelText('Date of birth (YYYY-MM-DD)')).toBeNull();
    await fireEvent.changeText(name, 'Ada L');
    await fireEvent.press(screen.getByRole('radio', { name: 'Non-binary' }));
    await press('Save');

    await waitFor(() => expect(screen).toHavePathname('/you'));
    expect(api.profile.saveProfile).toHaveBeenCalledWith({
      name: 'Ada L',
      dateOfBirth: '',
      gender: 'non-binary',
      languages: ['en', 'vi'],
      livesInCity: true,
      // Changing the profile never accepts a version: only the first save does.
      acceptedCodeOfConduct: false,
      onlyWomenAndNonBinary: true,
    });
  });

  it('shows that the code of conduct was accepted, without a box to tick, and sends no acceptance', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: saved });
    await open('/profile?next=/you', api);
    await screen.findByLabelText('The name you go by');
    expect(screen.getByText('You accepted the code of conduct.')).toBeTruthy();
    expect(screen.queryByRole('checkbox', { name: 'I agree to the code of conduct' })).toBeNull();
    await press('Save');
    await waitFor(() => expect(api.profile.saveProfile).toHaveBeenCalled());
    expect(api.profile.saveProfile.mock.calls[0][0]).toMatchObject({ acceptedCodeOfConduct: false });
  });

  it('offers to give the profile when there is none', async () => {
    await open('/you');
    expect(await screen.findByText('You have not given your profile yet.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('button', { name: 'Give your profile' }));
    await waitFor(() => expect(screen).toHavePathname('/profile'));
    expect(screen).toHaveSearchParams({ next: '/you' });
  });

  it('starts below the top inset, so the title is not under the status bar', async () => {
    const insets = jest.requireMock('react-native-safe-area-context').useSafeAreaInsets as jest.Mock;
    insets.mockReturnValue({ top: 40, bottom: 0, left: 0, right: 0 });
    await open('/you');
    await screen.findByText('You have not given your profile yet.');
    expect(StyleSheet.flatten(screen.getByTestId('screen').props.style).paddingTop).toBeGreaterThanOrEqual(40);
    insets.mockReturnValue({ top: 0, bottom: 0, left: 0, right: 0 });
  });
});
