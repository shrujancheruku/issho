import type { MiddlewareFunction, MiddlewareSettings } from 'expo-server';

export const unstable_settings: MiddlewareSettings = { matcher: { methods: ['GET'], patterns: ['/'] } };

/**
 * Plans opens first for a visitor (docs/spec/app-shape.md), and the screen that
 * says so (`index.tsx`) redirects in the client, which leaves a visitor with no
 * JavaScript an empty shell. The server sends them on instead. Runs on the
 * Expo server only; a phone never sees it.
 */
// A relative Location: behind the proxy the request's own origin is not the visitor's.
const middleware: MiddlewareFunction = () => new Response(null, { status: 302, headers: { Location: '/plans' } });
export default middleware;
