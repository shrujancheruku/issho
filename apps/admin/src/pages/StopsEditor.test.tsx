import { create } from '@bufbuild/protobuf';
import {
  Code, ConnectError, FindPlanPlacesResponseSchema, FoundPlaceSchema, GetReviewPlanResponseSchema, PlanStepSchema, ReviewPlanSchema,
  ReviewStopSchema, SetPlanStopsResponseSchema,
} from '@issho/api-client';
import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { StopsEditor } from './StopsEditor';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const planId = '44444444-4444-4444-4444-444444444444';

/** A walk of n stops: p1 (meet) ... pn (finish), named "Place k". */
function walk(n: number, upcomingEvents = 0) {
  const ids = Array.from({ length: n }, (_, i) => `p${i + 1}`);
  return create(GetReviewPlanResponseSchema, {
    plan: create(ReviewPlanSchema, { id: planId, walk: true }),
    steps: ids.map((_, i) => create(PlanStepSchema, { venueName: `Place ${i + 1}` })),
    stops: ids.map((id, i) => create(ReviewStopSchema, { placeId: id, minutes: 10 * (i + 1), note: `note ${i + 1}` })),
    upcomingEvents,
  });
}

const bodyRows = () => screen.getAllByRole('row').slice(1);
const names = () => bodyRows().map((r) => within(r).getAllByRole('cell')[1].textContent);
const roles = () => bodyRows().map((r) => within(r).getAllByRole('cell')[0].textContent);

describe('editing a walk', () => {
  it('labels meet, stops and finish by position', () => {
    renderPage(<StopsEditor data={walk(4)} onDone={() => {}} />);
    expect(roles()).toEqual(['Meet', 'Stop 1', 'Stop 2', 'Finish']);
  });

  it('reorders with the arrows (first up and last down are disabled) and sends the new order', async () => {
    fake.on('review.setPlanStops', () => create(SetPlanStopsResponseSchema, {}));
    const { user } = renderPage(<StopsEditor data={walk(3)} onDone={() => {}} />);
    expect((within(bodyRows()[0]).getByRole('button', { name: 'Move up' }) as HTMLButtonElement).disabled).toBe(true);
    expect((within(bodyRows()[2]).getByRole('button', { name: 'Move down' }) as HTMLButtonElement).disabled).toBe(true);

    await user.click(within(bodyRows()[0]).getByRole('button', { name: 'Move down' }));
    expect(names()).toEqual(['Place 2', 'Place 1', 'Place 3']);
    await user.click(screen.getByRole('button', { name: 'Save the walk' }));

    expect(fake.requests('review.setPlanStops')).toEqual([{
      planId,
      stops: [
        { placeId: 'p2', minutes: 20, note: 'note 2' },
        { placeId: 'p1', minutes: 10, note: 'note 1' },
        { placeId: 'p3', minutes: 30, note: 'note 3' },
      ],
    }]);
  });

  it('removes a stop, but never below three', async () => {
    const { user } = renderPage(<StopsEditor data={walk(4)} onDone={() => {}} />);
    await user.click(within(bodyRows()[1]).getByRole('button', { name: 'Remove' }));
    expect(names()).toEqual(['Place 1', 'Place 3', 'Place 4']);
    for (const b of screen.getAllByRole('button', { name: 'Remove' })) expect((b as HTMLButtonElement).disabled).toBe(true);
  });

  it('sends edited minutes and notes', async () => {
    fake.on('review.setPlanStops', () => create(SetPlanStopsResponseSchema, {}));
    const { user } = renderPage(<StopsEditor data={walk(3)} onDone={() => {}} />);
    const row = bodyRows()[1];
    const minutes = within(row).getByRole('spinbutton');
    await user.clear(minutes);
    await user.type(minutes, '45');
    const note = within(row).getByRole('textbox');
    await user.clear(note);
    await user.type(note, 'tea');
    await user.click(screen.getByRole('button', { name: 'Save the walk' }));
    expect((fake.requests('review.setPlanStops')[0] as { stops: unknown[] }).stops[1]).toEqual({ placeId: 'p2', minutes: 45, note: 'tea' });
  });

  it('finds a place and adds it before the finish; a place already in the walk cannot be added twice', async () => {
    const found = (placeId: string, name: string, extra = {}) => create(FoundPlaceSchema, { placeId, name, address: 'addr', meters: 500, ...extra });
    fake
      .on('review.findPlanPlaces', () => create(FindPlanPlacesResponseSchema, {
        places: [found('p1', 'Place 1'), found('new', 'New cafe'), found('far', 'Far away', { meters: 4000 }), found('shut', 'Shut', { closed: true })],
      }))
      .on('review.setPlanStops', () => create(SetPlanStopsResponseSchema, {}));
    const { user } = renderPage(<StopsEditor data={walk(3)} onDone={() => {}} />);
    await user.type(screen.getByPlaceholderText(/Find a place/), '  cafe ');
    await user.click(screen.getByRole('button', { name: 'Find' }));
    expect(fake.requests('review.findPlanPlaces')).toEqual([{ planId, query: 'cafe' }]);

    expect(await screen.findByText('In the walk')).toBeTruthy();
    expect(screen.getByText(/too far/)).toBeTruthy();
    expect(screen.getByText('closed')).toBeTruthy();
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(names()).toEqual(['Place 1', 'Place 2', 'New cafe', 'Place 3']);
    await user.click(screen.getByRole('button', { name: 'Save the walk' }));
    expect(fake.requests('review.setPlanStops')).toEqual([{
      planId,
      stops: [
        { placeId: 'p1', minutes: 10, note: 'note 1' },
        { placeId: 'p2', minutes: 20, note: 'note 2' },
        { placeId: 'new', minutes: 20, note: '' },
        { placeId: 'p3', minutes: 30, note: 'note 3' },
      ],
    }]);
  });

  it('does not search for a blank query', async () => {
    const { user } = renderPage(<StopsEditor data={walk(3)} onDone={() => {}} />);
    expect((screen.getByRole('button', { name: 'Find' }) as HTMLButtonElement).disabled).toBe(true);
    await user.type(screen.getByPlaceholderText(/Find a place/), '   {Enter}');
    expect(fake.calls).toEqual([]);
  });

  it('shows the saved time and the warnings, and the error when saving fails', async () => {
    fake.on('review.setPlanStops', () => create(SetPlanStopsResponseSchema, { minutes: 135, warnings: ['Too long'] }));
    const { user } = renderPage(<StopsEditor data={walk(3)} onDone={() => {}} />);
    await user.click(screen.getByRole('button', { name: 'Save the walk' }));
    expect(await screen.findByText('Saved: about 2 h 15 min in all.')).toBeTruthy();
    expect(screen.getByText('Too long')).toBeTruthy();

    fake.on('review.setPlanStops', () => { throw new ConnectError('route failed', Code.FailedPrecondition); });
    await user.click(screen.getByRole('button', { name: 'Save the walk' }));
    expect(await screen.findByText(/route failed/)).toBeTruthy();
  });

  it('warns when outings are already scheduled', () => {
    renderPage(<StopsEditor data={walk(3, 2)} onDone={() => {}} />);
    expect(screen.getByText(/2 outings of this plan are already scheduled/)).toBeTruthy();
  });

  it('calls onDone', async () => {
    const onDone = vi.fn();
    const { user } = renderPage(<StopsEditor data={walk(3)} onDone={onDone} />);
    await user.click(screen.getByRole('button', { name: 'Done' }));
    expect(onDone).toHaveBeenCalledOnce();
  });
});
