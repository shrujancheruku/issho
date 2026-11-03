import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactElement } from 'react';

/** Renders a page with a fresh query cache that never retries, so an error shows at once. */
export function renderPage(page: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
  return { user: userEvent.setup(), ...render(<QueryClientProvider client={client}>{page}</QueryClientProvider>) };
}
