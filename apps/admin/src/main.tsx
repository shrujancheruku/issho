import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from './App';
// Fonts are bundled rather than loaded from Google: the CSP allows only 'self'.
// Each CSS file is split by script (unicode-range), so a page loads only the
// Japanese or Latin files its text needs.
import '@fontsource/dela-gothic-one/400.css';
import '@fontsource/zen-kaku-gothic-new/400.css';
import '@fontsource/zen-kaku-gothic-new/700.css';
import './styles.css';

const queryClient = new QueryClient({
  // Moderators act on what they see: refetch when they come back to the tab.
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: true } },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
