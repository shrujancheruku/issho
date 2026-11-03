import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, GetReviewPlanResponseSchema, ReviewPlanSchema, ReviewPlanStatus } from '@issho/api-client';
import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { deferred } from '../test/deferred';
import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { PlanDetail } from './PlanDetail';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const planId = '33333333-3333-3333-3333-333333333333';

/** One plan the server holds; the decisions move it between statuses. */
function serve(status: ReviewPlanStatus) {
  const state = { status, closedReason: '' };
  const close = (reason: string) => () => { state.status = ReviewPlanStatus.CLOSED; state.closedReason = reason; return {}; };
  fake
    .on('review.getReviewPlan', () => create(GetReviewPlanResponseSchema, {
      plan: create(ReviewPlanSchema, { id: planId, title: 'Ramen night', storedTitle: 'Ramen night', summary: 'Noodles', status: state.status, closedReason: state.closedReason }),
    }))
    .on('review.editPlan', () => ({}))
    .on('review.approvePlan', () => { state.status = ReviewPlanStatus.OPEN; return {}; })
    .on('review.rejectPlan', close('rejected'))
    .on('review.withdrawPlan', close('withdrawn'));
  return state;
}

// Neither the plan's approval nor its rejection has a confirm step in the
// page, so there are no cancel tests; the double-click tests cover a second send.
describe('approve a draft plan', () => {
  it('sends the plan id alone when the text is unchanged, and shows it open', async () => {
    serve(ReviewPlanStatus.DRAFT);
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve' }));

    expect(fake.requests('review.approvePlan')).toEqual([{ planId }]);
    expect(fake.requests('review.editPlan')).toEqual([]);
    expect(await screen.findByText('Open')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Approve' })).toBeNull();
  });

  it('saves an edited title first, then approves', async () => {
    serve(ReviewPlanStatus.DRAFT);
    const { user } = renderPage(<PlanDetail planId={planId} />);
    const title = await screen.findByLabelText(/Title/);
    await user.clear(title);
    await user.type(title, 'Late ramen');
    await user.click(screen.getByRole('button', { name: 'Save and approve' }));

    expect(await screen.findByText('Open')).toBeTruthy();
    expect(fake.calls.map((c) => c.rpc).filter((r) => r !== 'review.getReviewPlan')).toEqual(['review.editPlan', 'review.approvePlan']);
    expect(fake.requests('review.editPlan')).toEqual([{ planId, title: 'Late ramen', summary: 'Noodles' }]);
  });

  it('shows the error and keeps the draft when the RPC fails', async () => {
    serve(ReviewPlanStatus.DRAFT);
    fake.on('review.approvePlan', () => { throw new ConnectError('the plan has changed', Code.FailedPrecondition); });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve' }));

    expect(await screen.findByText(/the plan has changed/)).toBeTruthy();
    expect(screen.getByText('Draft')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve(ReviewPlanStatus.DRAFT);
    const gate = deferred();
    fake.on('review.approvePlan', async () => { await gate.promise; state.status = ReviewPlanStatus.OPEN; return {}; });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    const approve = await screen.findByRole('button', { name: 'Approve' });
    await user.click(approve);
    await user.click(approve);
    gate.release();

    expect(await screen.findByText('Open')).toBeTruthy();
    expect(fake.requests('review.approvePlan')).toHaveLength(1);
  });
});

describe('reject a draft plan', () => {
  it('sends the plan id and shows the plan closed', async () => {
    serve(ReviewPlanStatus.DRAFT);
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Reject' }));

    expect(fake.requests('review.rejectPlan')).toEqual([{ planId }]);
    expect(await screen.findByText('Closed: rejected')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Reject' })).toBeNull();
    expect(fake.requests('review.approvePlan')).toEqual([]);
  });

  it('shows the error and keeps the draft when the RPC fails', async () => {
    serve(ReviewPlanStatus.DRAFT);
    fake.on('review.rejectPlan', () => { throw new ConnectError('no connection', Code.Unavailable); });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Reject' }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
    expect(screen.getByText('Draft')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve(ReviewPlanStatus.DRAFT);
    const gate = deferred();
    fake.on('review.rejectPlan', async () => { await gate.promise; state.status = ReviewPlanStatus.CLOSED; state.closedReason = 'rejected'; return {}; });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    const reject = await screen.findByRole('button', { name: 'Reject' });
    await user.click(reject);
    await user.click(reject);
    gate.release();

    expect(await screen.findByText('Closed: rejected')).toBeTruthy();
    expect(fake.requests('review.rejectPlan')).toHaveLength(1);
  });
});

describe('withdraw an open plan', () => {
  it('sends the plan id and shows the plan closed', async () => {
    serve(ReviewPlanStatus.OPEN);
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Withdraw plan' }));

    expect(fake.requests('review.withdrawPlan')).toEqual([{ planId }]);
    expect(await screen.findByText('Closed: withdrawn')).toBeTruthy();
  });

  it('shows the error and keeps the plan open when the RPC fails', async () => {
    serve(ReviewPlanStatus.OPEN);
    fake.on('review.withdrawPlan', () => { throw new ConnectError('no connection', Code.Unavailable); });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Withdraw plan' }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
    expect(screen.getByText('Open')).toBeTruthy();
  });
});
