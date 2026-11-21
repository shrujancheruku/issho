import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { type Api, type CheckResponse, CheckResponseSchema } from '@issho/api-client';

import { HealthPage } from './HealthPage';

afterEach(cleanup);

// A fake of the generated client: only the health check the page calls, and
// it answers with the generated response message.
function fakeApi(check: () => Promise<CheckResponse>): Api {
  return { health: { check } } as unknown as Api;
}

describe('HealthPage', () => {
  it('prints the version the API reports', async () => {
    const api = fakeApi(async () => create(CheckResponseSchema, { version: '1.2.3' }));

    render(<HealthPage api={api} />);

    expect(await screen.findByText('API version 1.2.3')).toBeTruthy();
  });

  it('says the API could not be reached when the check fails', async () => {
    const api = fakeApi(async () => {
      throw new Error('network down');
    });

    render(<HealthPage api={api} />);

    expect(await screen.findByText('The API could not be reached.')).toBeTruthy();
  });
});
