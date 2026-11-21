import type { Api } from '@issho/api-client';
import { createContext, useContext, type ReactNode } from 'react';

const ApiContext = createContext<Api | null>(null);

/** Gives screens the one generated client; tests give them a fake (test-support/fake-api.ts). */
export function ApiProvider({ api, children }: { api: Api; children: ReactNode }) {
  return <ApiContext.Provider value={api}>{children}</ApiContext.Provider>;
}

export function useApi(): Api {
  const api = useContext(ApiContext);
  if (!api) throw new Error('useApi needs an ApiProvider above it');
  return api;
}
