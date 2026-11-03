import { create } from '@bufbuild/protobuf';
import { ListReportsResponseSchema, ModerationReportSchema, ReportStatus, TimeBlock } from '@issho/api-client';
import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { Reports } from './Reports';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const report = (id: string) => create(ModerationReportSchema, { id, reportedName: `Person ${id}`, status: ReportStatus.OPEN });

// The paging convention (AIP-158) is the same on every admin list; Reports
// stands for them: each page's next_page_token is the next request's
// page_token, and "Load more" is there only while there is one.
describe('paging a list', () => {
  it('loads the next page with the token from the last one, then stops', async () => {
    fake.on('moderation.listReports', (req: { pageToken: string }) => (
      req.pageToken === ''
        ? create(ListReportsResponseSchema, { reports: [report('r1'), report('r2')], nextPageToken: 'after-r2' })
        : create(ListReportsResponseSchema, { reports: [report('r3')] })
    ));
    const { user } = renderPage(<Reports />);

    expect(await screen.findByText('Person r1')).toBeTruthy();
    expect(screen.queryByText('Person r3')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Load more' }));

    expect(await screen.findByText('Person r3')).toBeTruthy();
    expect(screen.getByText('Person r1')).toBeTruthy(); // the first page stays
    expect(fake.requests('moderation.listReports')).toEqual([
      { status: ReportStatus.OPEN, reportedId: '', pageToken: '' },
      { status: ReportStatus.OPEN, reportedId: '', pageToken: 'after-r2' },
    ]);
    expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull();
  });

  it('names the outing\'s part of the day', async () => {
    fake.on('moderation.listReports', () => create(ListReportsResponseSchema, {
      reports: [create(ModerationReportSchema, { id: 'r1', reportedName: 'Person r1', eventId: 'e1', eventTitle: 'Ramen', eventDate: '2030-01-06', eventBlock: TimeBlock.AFTERNOON })],
    }));
    renderPage(<Reports />);
    expect(await screen.findByText(/Outing: Ramen · 2030-01-06 afternoon/)).toBeTruthy();
  });

  it('shows no button when everything fits in one page', async () => {
    fake.on('moderation.listReports', () => create(ListReportsResponseSchema, { reports: [report('r1')] }));
    renderPage(<Reports />);
    expect(await screen.findByText('Person r1')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull();
  });
});
