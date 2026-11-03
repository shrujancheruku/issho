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

// Every decision asks first (ConfirmButton): the first click sends nothing.
describe('approve a draft plan', () => {
  it('sends the plan id alone when the text is unchanged, and shows it open', async () => {
    serve(ReviewPlanStatus.DRAFT);
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve' }));
    await user.click(screen.getByRole('button', { name: 'Yes, open this plan' }));

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
    expect(fake.calls.map((c) => c.rpc)).toEqual(['review.getReviewPlan']); // nothing saved until confirmed
    await user.click(screen.getByRole('button', { name: 'Yes, open this plan' }));

    expect(await screen.findByText('Open')).toBeTruthy();
    expect(fake.calls.map((c) => c.rpc).filter((r) => r !== 'review.getReviewPlan')).toEqual(['review.editPlan', 'review.approvePlan']);
    expect(fake.requests('review.editPlan')).toEqual([{ planId, title: 'Late ramen', summary: 'Noodles' }]);
  });

  it('shows the error and keeps the draft when the RPC fails', async () => {
    serve(ReviewPlanStatus.DRAFT);
    fake.on('review.approvePlan', () => { throw new ConnectError('the plan has changed', Code.FailedPrecondition); });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve' }));
    await user.click(screen.getByRole('button', { name: 'Yes, open this plan' }));

    expect(await screen.findByText(/the plan has changed/)).toBeTruthy();
    expect(screen.getByText('Draft')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve(ReviewPlanStatus.DRAFT);
    const gate = deferred();
    fake.on('review.approvePlan', async () => { await gate.promise; state.status = ReviewPlanStatus.OPEN; return {}; });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Approve' }));
    const yes = screen.getByRole('button', { name: 'Yes, open this plan' });
    await user.click(yes);
    await user.click(yes); // the RPC hasn't answered yet
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
    await user.click(screen.getByRole('button', { name: 'Yes, reject this plan' }));

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
    await user.click(screen.getByRole('button', { name: 'Yes, reject this plan' }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
    expect(screen.getByText('Draft')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve(ReviewPlanStatus.DRAFT);
    const gate = deferred();
    fake.on('review.rejectPlan', async () => { await gate.promise; state.status = ReviewPlanStatus.CLOSED; state.closedReason = 'rejected'; return {}; });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Reject' }));
    const yes = screen.getByRole('button', { name: 'Yes, reject this plan' });
    await user.click(yes);
    await user.click(yes); // the RPC hasn't answered yet
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
    await user.click(screen.getByRole('button', { name: 'Yes, withdraw this plan' }));

    expect(fake.requests('review.withdrawPlan')).toEqual([{ planId }]);
    expect(await screen.findByText('Closed: withdrawn')).toBeTruthy();
  });

  it('shows the error and keeps the plan open when the RPC fails', async () => {
    serve(ReviewPlanStatus.OPEN);
    fake.on('review.withdrawPlan', () => { throw new ConnectError('no connection', Code.Unavailable); });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Withdraw plan' }));
    await user.click(screen.getByRole('button', { name: 'Yes, withdraw this plan' }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
    expect(screen.getByText('Open')).toBeTruthy();
  });
});

describe('cancelling a confirmation', () => {
  it.each([
    [ReviewPlanStatus.DRAFT, 'Approve', 'Draft', 'review.approvePlan'],
    [ReviewPlanStatus.DRAFT, 'Reject', 'Draft', 'review.rejectPlan'],
    [ReviewPlanStatus.OPEN, 'Withdraw plan', 'Open', 'review.withdrawPlan'],
  ] as const)('%#: sends nothing and leaves the plan as it was', async (status, label, badge, rpc) => {
    serve(status);
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: label }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(fake.requests(rpc)).toEqual([]);
    expect(screen.getByRole('button', { name: label })).toBeTruthy();
    expect(screen.getByText(badge)).toBeTruthy();
  });
});

describe('withdraw double submit', () => {
  it('sends one RPC for a double click', async () => {
    const state = serve(ReviewPlanStatus.OPEN);
    const gate = deferred();
    fake.on('review.withdrawPlan', async () => { await gate.promise; state.status = ReviewPlanStatus.CLOSED; state.closedReason = 'withdrawn'; return {}; });
    const { user } = renderPage(<PlanDetail planId={planId} />);
    await user.click(await screen.findByRole('button', { name: 'Withdraw plan' }));
    const yes = screen.getByRole('button', { name: 'Yes, withdraw this plan' });
    await user.click(yes);
    await user.click(yes);
    gate.release();

    expect(await screen.findByText('Closed: withdrawn')).toBeTruthy();
    expect(fake.requests('review.withdrawPlan')).toHaveLength(1);
  });
});
