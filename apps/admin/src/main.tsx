import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { createApi } from '@issho/api-client';

import { HealthPage } from './HealthPage';

// An empty base URL means the API is on this origin; in development Vite
// proxies the API's paths to the local API (see vite.config.ts), so the browser
// makes no cross-origin call.
const api = createApi({
  baseUrl: import.meta.env.VITE_API_URL ?? '',
  getAccessToken: async () => null,
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <HealthPage api={api} />
  </StrictMode>,
);
