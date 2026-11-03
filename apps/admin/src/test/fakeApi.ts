import type { createApi } from '@issho/api-client';

type Api = ReturnType<typeof createApi>;
/** "review.approvePlan": every RPC the admin SPA can call, as service.method. */
export type RpcName = {
  [S in keyof Api]: Api[S] extends Record<string, unknown> ? `${S & string}.${keyof Api[S] & string}` : never;
}[keyof Api];

export type Call = { rpc: string; request: unknown };

type Answer = (request: never) => unknown;

/**
 * A stand-in for the `api` export of src/lib.ts, faked at the
 * `@issho/api-client` boundary (no fetch). Every call is recorded with its
 * exact request; `on` sets what an RPC answers (a function that may throw or
 * return a promise). An RPC with no answer fails the call, so a page can't
 * quietly call something the test didn't expect.
 */
export class FakeApi {
  readonly calls: Call[] = [];
  private answers = new Map<string, Answer>();

  /** The object pages import as `api`. */
  readonly api = new Proxy({}, {
    get: (_, service: string) => new Proxy({}, {
      get: (__, method: string) => async (request: unknown) => {
        const rpc = `${service}.${method}`;
        this.calls.push({ rpc, request });
        const answer = this.answers.get(rpc);
        if (!answer) throw new Error(`unexpected RPC ${rpc}`);
        return answer(request as never);
      },
    }),
  }) as Api;

  on(rpc: RpcName, answer: Answer): this {
    this.answers.set(rpc, answer);
    return this;
  }

  /** The requests sent to one RPC, in order. */
  requests(rpc: RpcName): unknown[] {
    return this.calls.filter((c) => c.rpc === rpc).map((c) => c.request);
  }

  reset() {
    this.calls.length = 0;
    this.answers.clear();
  }
}

/** One shared instance: vi.mock('../lib') hands its `api` to the pages. */
export const fake = new FakeApi();
