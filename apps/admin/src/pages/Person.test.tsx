import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, EventStatus, GetPersonResponseSchema, InviteResponse, PersonOutingSchema, PersonSchema, TimeBlock } from '@issho/api-client';
import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { deferred } from '../test/deferred';
import { renderPage } from '../test/render';
import { Person } from './Person';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const userId = '11111111-1111-1111-1111-111111111111';

/** The person the server holds; suspend/lift flip it, as the real RPCs would. */
function serve(suspended: boolean) {
  const state = { suspended };
  fake
    .on('moderation.getPerson', () => create(GetPersonResponseSchema, { person: create(PersonSchema, { userId, name: 'Aoi', suspended: state.suspended }) }))
    .on('moderation.suspendPerson', () => { state.suspended = true; return {}; })
    .on('moderation.liftSuspension', () => { state.suspended = false; return {}; });
  return state;
}

describe('outings', () => {
  it('shows the block, a status unless done, and the answer as the same words as before the enums', async () => {
    fake.on('moderation.getPerson', () => create(GetPersonResponseSchema, {
      person: create(PersonSchema, {
        userId, name: 'Aoi',
        outings: [
          create(PersonOutingSchema, { eventId: 'e1', title: 'Ramen', date: '2030-01-06', block: TimeBlock.EVENING, eventStatus: EventStatus.CANCELLED, response: InviteResponse.DROPPED }),
          create(PersonOutingSchema, { eventId: 'e2', title: 'Walk', date: '2030-01-07', block: TimeBlock.MORNING, eventStatus: EventStatus.DONE, response: InviteResponse.CONFIRMED }),
        ],
      }),
    }));
    renderPage(<Person userId={userId} />);
    expect(await screen.findByText('2030-01-06 evening')).toBeTruthy();
    expect(screen.getByText('(cancelled)')).toBeTruthy();
    expect(screen.getByText('dropped')).toBeTruthy();
    expect(screen.getByText('2030-01-07 morning')).toBeTruthy();
    expect(screen.queryByText('(done)')).toBeNull();
  });
});

describe('suspend a person', () => {
  it('asks to confirm, sends the reason once confirmed, and shows the suspension', async () => {
    serve(false);
    const { user } = renderPage(<Person userId={userId} />);
    await user.type(await screen.findByLabelText('Reason'), 'harassed someone');

    await user.click(screen.getByRole('button', { name: 'Suspend…' }));
    expect(fake.requests('moderation.suspendPerson')).toEqual([]); // the first click only asks
    await user.click(screen.getByRole('button', { name: 'Yes, suspend Aoi' }));

    expect(fake.requests('moderation.suspendPerson')).toEqual([{ userId, reason: 'harassed someone' }]);
    expect(await screen.findByText('Suspended')).toBeTruthy();
    expect(screen.getByText('Lift the suspension')).toBeTruthy();
  });

  it('sends nothing when the confirmation is cancelled', async () => {
    serve(false);
    const { user } = renderPage(<Person userId={userId} />);
    await user.type(await screen.findByLabelText('Reason'), 'spam');
    await user.click(screen.getByRole('button', { name: 'Suspend…' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(fake.requests('moderation.suspendPerson')).toEqual([]);
    expect(screen.getByRole('button', { name: 'Suspend…' })).toBeTruthy(); // back to the first step
  });

  it('needs a reason before it offers to suspend', async () => {
    serve(false);
    renderPage(<Person userId={userId} />);
    const button = await screen.findByRole('button', { name: 'Suspend…' });
    expect((button as HTMLButtonElement).disabled).toBe(true);
  });

  it('shows the error and stays on the confirmation when the RPC fails', async () => {
    serve(false);
    fake.on('moderation.suspendPerson', () => { throw new ConnectError('not an admin', Code.PermissionDenied); });
    const { user } = renderPage(<Person userId={userId} />);
    await user.type(await screen.findByLabelText('Reason'), 'spam');
    await user.click(screen.getByRole('button', { name: 'Suspend…' }));
    await user.click(screen.getByRole('button', { name: 'Yes, suspend Aoi' }));

    expect(await screen.findByText(/not an admin/)).toBeTruthy();
    expect(screen.queryByText('Suspended')).toBeNull();
    expect(screen.getByRole('button', { name: 'Yes, suspend Aoi' })).toBeTruthy(); // can retry or cancel
  });

  it('sends one RPC for a double click', async () => {
    const state = serve(false);
    const gate = deferred();
    fake.on('moderation.suspendPerson', async () => { await gate.promise; state.suspended = true; return {}; });
    const { user } = renderPage(<Person userId={userId} />);
    await user.type(await screen.findByLabelText('Reason'), 'spam');
    await user.click(screen.getByRole('button', { name: 'Suspend…' }));
    const yes = screen.getByRole('button', { name: 'Yes, suspend Aoi' });
    await user.click(yes);
    expect((yes as HTMLButtonElement).disabled).toBe(true); // in flight
    await user.click(yes);
    expect(fake.requests('moderation.suspendPerson')).toHaveLength(1);
    gate.release();
    expect(await screen.findByText('Suspended')).toBeTruthy();
    expect(fake.requests('moderation.suspendPerson')).toHaveLength(1);
  });
});

describe('lift a suspension', () => {
  it('sends the note and shows the person as no longer suspended', async () => {
    serve(true);
    const { user } = renderPage(<Person userId={userId} />);
    await user.type(await screen.findByLabelText('Note'), 'appealed');
    await user.click(screen.getByRole('button', { name: 'Lift suspension' }));
    expect(fake.requests('moderation.liftSuspension')).toEqual([]); // the first click only asks
    await user.click(screen.getByRole('button', { name: "Yes, lift Aoi's suspension" }));

    expect(fake.requests('moderation.liftSuspension')).toEqual([{ userId, note: 'appealed' }]);
    expect(await screen.findByText('Suspend', { selector: 'h2' })).toBeTruthy();
    expect(screen.queryByText('Suspended')).toBeNull();
  });

  it('sends nothing when the confirmation is cancelled', async () => {
    serve(true);
    const { user } = renderPage(<Person userId={userId} />);
    await user.click(await screen.findByRole('button', { name: 'Lift suspension' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(fake.requests('moderation.liftSuspension')).toEqual([]);
    expect(screen.getByRole('button', { name: 'Lift suspension' })).toBeTruthy();
    expect(screen.getByText('Suspended')).toBeTruthy();
  });

  it('shows the error and keeps the suspension when the RPC fails', async () => {
    serve(true);
    fake.on('moderation.liftSuspension', () => { throw new ConnectError('try again later', Code.Unavailable); });
    const { user } = renderPage(<Person userId={userId} />);
    await user.click(await screen.findByRole('button', { name: 'Lift suspension' }));
    await user.click(screen.getByRole('button', { name: "Yes, lift Aoi's suspension" }));

    expect(await screen.findByText(/try again later/)).toBeTruthy();
    expect(screen.getByText('Suspended')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve(true);
    const gate = deferred();
    fake.on('moderation.liftSuspension', async () => { await gate.promise; state.suspended = false; return {}; });
    const { user } = renderPage(<Person userId={userId} />);
    await user.click(await screen.findByRole('button', { name: 'Lift suspension' }));
    await user.click(screen.getByRole('button', { name: "Yes, lift Aoi's suspension" }));
    expect(screen.queryByRole('button', { name: "Yes, lift Aoi's suspension" })).toBeNull(); // folded back
    const again = screen.getByRole('button', { name: 'Lift suspension' });
    expect((again as HTMLButtonElement).disabled).toBe(true); // the RPC hasn't answered yet
    await user.click(again);
    expect(fake.requests('moderation.liftSuspension')).toHaveLength(1);
    gate.release();
    expect(await screen.findByText('Suspend', { selector: 'h2' })).toBeTruthy();
    expect(fake.requests('moderation.liftSuspension')).toHaveLength(1);
  });
});
