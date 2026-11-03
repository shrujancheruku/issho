import { create } from '@bufbuild/protobuf';
import {
  Code, ConnectError, GetReviewEventResponseSchema, ReviewEventSchema, ReviewEventStatus, ReviewGroupSchema,
} from '@issho/api-client';
import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { deferred } from '../test/deferred';
import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { GroupReview } from './GroupReview';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const eventId = '22222222-2222-2222-2222-222222222222';

/** A proposed round that approve turns into a scheduled one, and reject deletes. */
function serve() {
  const state = { status: ReviewEventStatus.PROPOSED, gone: false };
  fake
    .on('review.getReviewEvent', () => {
      if (state.gone) throw new ConnectError('no such event', Code.NotFound);
      return create(GetReviewEventResponseSchema, {
        event: create(ReviewEventSchema, { id: eventId, title: 'Ramen night', status: state.status }),
        groups: [create(ReviewGroupSchema, { id: 'g1' })],
      });
    })
    .on('review.approveProposal', () => { state.status = ReviewEventStatus.SCHEDULED; return {}; })
    .on('review.rejectProposal', () => { state.gone = true; return {}; });
  return state;
}

// The page has no confirm step for either decision: approve sends the
// invitations at the first click. So there is no cancel test; the double-click
// tests below are what stops a second send.
describe('approve a proposed round', () => {
  it('sends the event id and shows the round as scheduled', async () => {
    serve();
    const { user } = renderPage(<GroupReview eventId={eventId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve and send invitations' }));

    expect(fake.requests('review.approveProposal')).toEqual([{ eventId }]);
    expect(await screen.findByText('Scheduled')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Approve and send invitations' })).toBeNull();
    expect(screen.getByText('Saved.')).toBeTruthy();
  });

  it('shows the error and keeps the proposal when the RPC fails', async () => {
    serve();
    fake.on('review.approveProposal', () => { throw new ConnectError('a group is below its minimum', Code.FailedPrecondition); });
    const { user } = renderPage(<GroupReview eventId={eventId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve and send invitations' }));

    expect(await screen.findByText(/below its minimum/)).toBeTruthy();
    expect(screen.getByText('Proposed')).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Approve and send invitations' }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('sends one RPC for a double click', async () => {
    const state = serve();
    const gate = deferred();
    fake.on('review.approveProposal', async () => { await gate.promise; state.status = ReviewEventStatus.SCHEDULED; return {}; });
    const { user } = renderPage(<GroupReview eventId={eventId} />);
    const approve = await screen.findByRole('button', { name: 'Approve and send invitations' });
    await user.click(approve);
    await user.click(approve);
    gate.release();

    expect(await screen.findByText('Scheduled')).toBeTruthy();
    expect(fake.requests('review.approveProposal')).toHaveLength(1);
  });
});

describe('reject a proposed round', () => {
  it('sends the event id and takes the decision buttons away', async () => {
    serve();
    const { user } = renderPage(<GroupReview eventId={eventId} />);
    await user.click(await screen.findByRole('button', { name: 'Reject' }));

    expect(fake.requests('review.rejectProposal')).toEqual([{ eventId }]);
    // The proposal is deleted, so the refetch finds nothing: the page says so.
    expect(await screen.findByText(/no such event/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Reject' })).toBeNull();
    expect(fake.requests('review.approveProposal')).toEqual([]);
  });

  it('shows the error and keeps the proposal when the RPC fails', async () => {
    serve();
    fake.on('review.rejectProposal', () => { throw new ConnectError('already decided', Code.FailedPrecondition); });
    const { user } = renderPage(<GroupReview eventId={eventId} />);
    await user.click(await screen.findByRole('button', { name: 'Reject' }));

    expect(await screen.findByText(/already decided/)).toBeTruthy();
    expect(screen.getByText('Proposed')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve();
    const gate = deferred();
    fake.on('review.rejectProposal', async () => { await gate.promise; state.gone = true; return {}; });
    const { user } = renderPage(<GroupReview eventId={eventId} />);
    const reject = await screen.findByRole('button', { name: 'Reject' });
    await user.click(reject);
    await user.click(reject);
    gate.release();

    expect(await screen.findByText(/no such event/)).toBeTruthy();
    expect(fake.requests('review.rejectProposal')).toHaveLength(1);
  });
});
