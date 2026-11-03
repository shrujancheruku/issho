import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

import { fake } from './fakeApi';

afterEach(() => {
  cleanup();
  fake.reset();
  localStorage.clear();
});
