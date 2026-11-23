import { expect, test, type BrowserContext, type Locator, type Page } from '@playwright/test';

import { appOrigin, call, formPlan, seed, serverClock, signInCode, signInWithoutBrowser } from '../support/stack';

// Journey 1, the core loop: three people each arrive by a join link, sign in with
// the code from the mail catcher, give a profile and sign up to one plan with a
// shared time; forming makes their outing; each says yes; the clock moves to the
// day, each checks in; the clock moves past the plan's end, and each gives
// feedback (docs/spec/quality.md, "End to end"). What each sees is asserted at
// every step, and while they are only invited, that nobody else's name is on
// their screen or in what the API sent it.
//
// The app and the API run on the same instants: the API's clock (and its
// worker's) is moved through the test-only RPC, and each browser's own clock to
// match, because the app works the check-in and feedback windows out from the
// device's time (apps/app/src/outings/model.ts).

const LANTERNS = '6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001';
const LANTERNS_TITLE = 'Hoi An old town at lantern time';
// 09:00 in Da Nang–Hoi An on Tuesday 1 December 2026, a day inside the fixture
// plan's round of 30 November to 6 December.
const START = new Date('2026-12-01T02:00:00Z');
const MINUTE = 60_000;

type Outing = {
  id: string;
  planId: string;
  date: string;
  seatState: string;
  outingState: string;
  othersGoing?: string[];
  checkedInNames?: string[];
  checkInOpensAt: string;
  checkInClosesAt: string;
  feedbackOpensAt: string;
};

class Person {
  readonly email: string;
  page!: Page;
  context!: BrowserContext;
  // What the page received from the API for the outings and the feedback form,
  // in the order it arrived: the privacy assertions read these as well as the text.
  private received: Promise<string>[] = [];

  constructor(
    readonly name: string,
    run: string,
  ) {
    this.email = `${name.toLowerCase()}-${run}@e2e.test`;
  }

  async open(browser: import('@playwright/test').Browser) {
    // Phone width, as the spec's screens are drawn.
    this.context = await browser.newContext({ viewport: { width: 390, height: 844 } });
    await this.context.clock.install({ time: START });
    this.page = await this.context.newPage();
    this.page.on('response', (res) => {
      if (/\/issho\.v1\.(OutingService\/(ListMyOutings|GetFeedback))/.test(res.url())) this.received.push(res.text().catch(() => ''));
    });
  }

  /** Everything the API has sent this page about outings since the last call. */
  async takeReceived(): Promise<string[]> {
    const bodies = await Promise.all(this.received);
    this.received = [];
    return bodies;
  }

  /** Loads a path and returns the outings the API sent for it. */
  async goto(path: string): Promise<Outing[]> {
    const listed = this.page.waitForResponse((r) => r.url().includes('/issho.v1.OutingService/ListMyOutings'));
    await this.page.goto(`${appOrigin()}${path}`);
    return ((await (await listed).json()) as { outings?: Outing[] }).outings ?? [];
  }

  async reload(): Promise<Outing[]> {
    const listed = this.page.waitForResponse((r) => r.url().includes('/issho.v1.OutingService/ListMyOutings'));
    await this.page.reload();
    return ((await (await listed).json()) as { outings?: Outing[] }).outings ?? [];
  }

  async screenText(): Promise<string> {
    return this.page.locator('body').innerText();
  }
}

/**
 * Picks a radio answer. The app marks a radio `selected`, which the web leaves off
 * the DOM, so Playwright's check() sees no change and nothing can be asserted of
 * the radio itself: the step that follows (or the saved answer) is the proof.
 */
async function choose(page: Page, name: string) {
  const radio = page.getByRole('radio', { name, exact: true });
  await radio.click();
}

/**
 * Ticks a checkbox. The app marks a checkbox `checked`, which the web leaves off
 * the DOM, so Playwright's check() sees no change; what the page does show is the
 * tick drawn before a ticked label, which is what a person reads.
 */
async function tick(box: Locator) {
  await box.click();
  await expect(box).toContainText('✓');
}

/**
 * Loads Together afresh for a person who is still only invited and asserts that
 * nobody else's name reaches them: not on the screen, not in any body the page
 * received, and `othersGoing` and `checkedInNames` empty. It is run for each
 * person still invited at the invited step and again after every yes, since that
 * is when a name could leak: others have said yes, this person has not.
 * Returns the outing the API sent.
 */
async function expectInvitedSeesNoNames(p: Person, others: string[]): Promise<Outing> {
  await p.takeReceived();
  const outings = await p.goto('/together');
  expect(outings).toHaveLength(1);
  const outing = outings[0];
  expect(outing.seatState).toBe('SEAT_STATE_INVITED');
  expect(outing.outingState).toBe('OUTING_STATE_OFFERED');

  const card = p.page.getByTestId(`needs-you-${outing.id}`);
  await expect(p.page.getByRole('heading', { name: 'Needs you' })).toBeVisible();
  await expect(card).toContainText(LANTERNS_TITLE);
  await expect(card).toContainText('You share: English');
  await expect(card.getByRole('button', { name: 'Yes' })).toBeVisible();

  // An invited person sees counts and languages, never a name, neither on the
  // page nor in what the API sent the page.
  const text = await p.screenText();
  for (const other of others) expect(text).not.toContain(other);
  const bodies = await p.takeReceived();
  expect(bodies.length).toBeGreaterThan(0);
  for (const body of bodies) {
    for (const other of others) expect(body).not.toContain(other);
    for (const o of (JSON.parse(body) as { outings?: Outing[] }).outings ?? []) {
      expect(o.othersGoing ?? []).toEqual([]);
      expect(o.checkedInNames ?? []).toEqual([]);
    }
  }
  return outing;
}

/** Moves the API's clock, its worker's, and every browser's to the same instant. */
async function moveClock(people: Person[], instant: Date) {
  await serverClock.set(instant);
  for (const p of people) await p.context.clock.setSystemTime(instant);
}

test('journey 1: from a join link to feedback, three people in one outing', async ({ browser }) => {
  const run = Math.random().toString(36).slice(2, 8);
  const [ana, ben, cai] = [new Person('Zephyr', run), new Person('Marigold', run), new Person('Quillon', run)];
  const everyone = [ana, ben, cai];
  const namesOf = (p: Person) => everyone.filter((o) => o !== p).map((o) => o.name);

  // Scene: the fixture loaded afresh (so a retry starts without the first try's
  // signups), the clock at the start, and the owner, admitted, with three links.
  await seed();
  await serverClock.set(START);
  const owner = `owner-${run}@e2e.test`;
  const ownerToken = await signInWithoutBrowser(owner);
  await call('issho.v1.AccountService', 'GetAccount', {}, ownerToken);
  await seed(owner);
  const codes: string[] = [];
  for (let i = 0; i < everyone.length; i++) {
    const made = await call<{ link: { code: string } }>('issho.v1.JoinLinkService', 'CreateJoinLink', {}, ownerToken);
    codes.push(made.link.code);
  }

  for (const p of everyone) await p.open(browser);

  await test.step('each arrives by a join link, signs in with the emailed code and is let in', async () => {
    for (const [i, p] of everyone.entries()) {
      const page = p.page;
      await page.goto(`${appOrigin()}/join/${codes[i]}`);
      // Signed out, the link leads to sign-in first.
      await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
      await page.getByLabel('Email').fill(p.email);
      await page.getByRole('button', { name: 'Send me a code' }).click();
      await expect(page.getByText(`We sent a 6-digit code to ${p.email}.`)).toBeVisible();
      await page.getByLabel('6-digit code').fill(await signInCode(p.email));
      await page.getByRole('button', { name: 'Sign in', exact: true }).click();
      await expect(page.getByRole('heading', { name: "You're in" })).toBeVisible();
      await expect(page.getByText('Welcome to Issho. You can now sign up to plans.')).toBeVisible();
    }
  });

  let sharedTime = '';
  await test.step('each opens the plan, gives a profile and signs up with the same time', async () => {
    for (const p of everyone) {
      const page = p.page;
      await page.getByRole('button', { name: 'See plans' }).click();
      await page.getByRole('link', { name: new RegExp(LANTERNS_TITLE) }).click();
      await expect(page.getByRole('heading', { name: LANTERNS_TITLE })).toBeVisible();

      // No profile yet: signing up asks for one first.
      await page.getByRole('button', { name: 'Sign up', exact: true }).click();
      await expect(page.getByText(/^Step 1 of 6$/)).toBeVisible();
      await page.getByLabel('The name you go by').fill(p.name);
      await page.getByRole('button', { name: 'Next' }).click();
      await page.getByLabel('Date of birth (YYYY-MM-DD)').fill('1990-06-15');
      await page.getByRole('button', { name: 'Next' }).click();
      await choose(page, 'Man');
      await page.getByRole('button', { name: 'Next' }).click();
      await tick(page.getByRole('checkbox', { name: 'English', exact: true }));
      await page.getByRole('button', { name: 'Next' }).click();
      await choose(page, 'Yes');
      await page.getByRole('button', { name: 'Next' }).click();
      await tick(page.getByRole('checkbox', { name: 'I agree to the code of conduct' }));
      await page.getByRole('button', { name: 'Save' }).click();

      // Back on the plan, the signup asks for times: the first evening on offer,
      // which the three share.
      await expect(page.getByRole('heading', { name: LANTERNS_TITLE })).toBeVisible();
      await page.getByRole('button', { name: 'Sign up', exact: true }).click();
      await expect(page.getByRole('heading', { name: 'When can you go?' })).toBeVisible();
      if (!sharedTime) sharedTime = (await page.getByRole('checkbox', { name: /evening$/ }).first().getAttribute('aria-label')) ?? '';
      expect(sharedTime).toMatch(/evening$/);
      await tick(page.getByRole('checkbox', { name: sharedTime, exact: true }));
      await page.getByRole('button', { name: 'Sign up', exact: true }).click();
      await expect(page.getByText(/^You signed up\. You will hear by /)).toBeVisible();
    }
  });

  let outing!: Outing;
  await test.step('forming invites each of the three, who see no names', async () => {
    await formPlan(LANTERNS);
    for (const p of everyone) {
      outing = await expectInvitedSeesNoNames(p, namesOf(p));
      // The sign-up's shared time is the outing's: the fixture plan runs in the evening.
      expect(outing.date).toMatch(/^2026-12-0[4-9]$/);
    }
  });

  await test.step('each says yes, and the third yes sets the outing', async () => {
    for (const [i, p] of everyone.entries()) {
      await p.takeReceived();
      await p.goto('/together');
      await p.page.getByTestId(`needs-you-${outing.id}`).getByRole('button', { name: 'Yes' }).click();
      if (i < 2) {
        // Held, not set: the answer is kept and the screen says what is awaited.
        await expect(p.page.getByTestId(`together-row-${outing.id}`)).toContainText('You said yes. It is set when 3 have.');
        // Still only a yes of their own: no names yet.
        const text = await p.screenText();
        for (const other of namesOf(p)) expect(text).not.toContain(other);
        // And everyone not yet answered, who now has others' yeses behind them,
        // reloads Together and still sees no name.
        for (const waiting of everyone.slice(i + 1)) await expectInvitedSeesNoNames(waiting, namesOf(waiting));
      } else {
        await expect(p.page.getByTestId(`together-row-${outing.id}`)).toContainText("You're going");
      }
    }
  });

  await test.step('those going see who they are going with, and check-in has not opened', async () => {
    for (const p of everyone) {
      await p.takeReceived();
      const outings = await p.goto(`/plans/${LANTERNS}`);
      const strip = p.page.getByTestId('outing-strip');
      await expect(strip).toContainText("You're going");
      await expect(strip).toContainText('Stage: Set');
      await expect(strip).toContainText('Going with:');
      for (const other of namesOf(p)) await expect(strip).toContainText(other);
      expect([...(outings[0].othersGoing ?? [])].sort()).toEqual([...namesOf(p)].sort());
      await expect(strip).toContainText('Check-in opens');
      await expect(strip.getByRole('button', { name: "I'm here" })).toHaveCount(0);
    }
    outing = (await ana.reload())[0];
  });

  await test.step('a minute before the day, check-in is still shut; when it opens, each checks in', async () => {
    const opens = new Date(outing.checkInOpensAt);
    await moveClock(everyone, new Date(opens.getTime() - MINUTE));
    for (const p of everyone) {
      await p.reload();
      const strip = p.page.getByTestId('outing-strip');
      await expect(strip).toContainText('Check-in opens');
      await expect(strip.getByRole('button', { name: "I'm here" })).toHaveCount(0);
    }

    // The instant it opens counts.
    await moveClock(everyone, opens);
    const alsoHere: string[][] = [[], [], []];
    for (const [i, p] of everyone.entries()) {
      await p.reload();
      const strip = p.page.getByTestId('outing-strip');
      await strip.getByRole('button', { name: "I'm here" }).click();
      await expect(strip).toContainText('You have checked in.');
      // Those who came before are named, and only them.
      if (i === 0) await expect(strip).toContainText('No one else has checked in yet.');
      else for (const before of everyone.slice(0, i)) await expect(strip).toContainText(before.name);
      alsoHere[i] = everyone.slice(0, i).map((q) => q.name);
    }
    // The first to arrive now sees the other two.
    await ana.reload();
    const strip = ana.page.getByTestId('outing-strip');
    await expect(strip).toContainText('Also here:');
    for (const other of namesOf(ana)) await expect(strip).toContainText(other);
    expect(alsoHere[2]).toEqual([ana.name, ben.name]);
  });

  await test.step('past the plan’s end, feedback opens and each gives theirs', async () => {
    await moveClock(everyone, new Date(new Date(outing.checkInClosesAt).getTime() + MINUTE));
    for (const [i, p] of everyone.entries()) {
      await p.goto('/together');
      const card = p.page.getByTestId(`needs-you-${outing.id}`);
      await expect(card).toContainText('How did it go? Feedback closes');
      await card.getByRole('link', { name: 'Give feedback' }).click();

      await expect(p.page.getByRole('heading', { name: 'How did it go?' })).toBeVisible();
      await choose(p.page, 'Yes');
      // Everyone who was going is ticked by default.
      const there = p.page.getByTestId('feedback-there');
      for (const other of namesOf(p)) await expect(there.getByRole('checkbox', { name: other })).toContainText('✓');
      // Each would meet the next person again.
      const pick = everyone[(i + 1) % everyone.length].name;
      await tick(p.page.getByTestId('feedback-again').getByRole('checkbox', { name: pick }));
      await p.page.getByRole('button', { name: 'Save my answers' }).click();
      await expect(p.page.getByText('You said you went. You can change your answers until feedback closes.')).toBeVisible();
    }
    // Nothing tells a person who picked them: after all three have answered, no
    // screen says so.
    for (const p of everyone) {
      await p.goto(`/plans/${LANTERNS}`);
      await expect(p.page.getByText('You said you went.')).toBeVisible();
      expect(await p.screenText()).not.toMatch(/picked you|chose you|wants to meet you/i);
    }
  });

  for (const p of everyone) await p.context.close();
});
