/// <reference types="jest" />
import { Code, ConnectError } from '@issho/api-client';

import { saveErrorMessage } from '../save-error';

const t = (k: string) => k;

it('never shows the server text for a rejected profile', () => {
  const err = new ConnectError('activities: pick at least 3', Code.InvalidArgument);
  expect(saveErrorMessage(err, t as never)).toBe('error.saveRejected');
});

it('words anything else as errorText does', () => {
  expect(saveErrorMessage(new ConnectError('x', Code.Unavailable), t as never)).toBe('error.network');
  expect(saveErrorMessage(new Error('boom'), t as never)).toBe('error.generic');
});
