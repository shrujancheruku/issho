import type { Api } from '@issho/api-client';

/**
 * A stand-in for the generated client (packages/api-client): every RPC is a
 * jest.fn that answers with what a test gives it, so a screen test never
 * reaches a network. Add each service's methods here as the screens start
 * calling them.
 */
export function fakeApi(): {
  [K in keyof Api]: { [M in keyof Api[K]]: jest.Mock };
} {
  return {
    health: { check: jest.fn().mockResolvedValue({ version: 'test' }) },
    plans: {
      listPlans: jest.fn().mockResolvedValue({ plans: [] }),
      getPlan: jest.fn(),
    },
    account: {
      getAccount: jest.fn().mockResolvedValue({
        account: {
          id: 'u1',
          email: 'ada@example.com',
          city: 'da-nang-hoi-an',
          displayLanguage: 'en',
        },
      }),
    },
  } as never;
}
