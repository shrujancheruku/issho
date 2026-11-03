// TanStack Query schedules a garbage-collection timer (gcTime, 5 minutes) for
// every query a test creates, and keeps no handle to cancel it. Those timers
// kept Jest's process alive after the last test: 17s of tests, then ~5 minutes
// of waiting for the timers to fire (CI run 41663137918; `--detectOpenHandles`
// lists them as "Timeout" at query-core/timeoutManager.ts). unref() lets the
// process exit once the tests are done, and changes nothing while they run.
// The globals are looked up per call so fake timers still take over.
const { timeoutManager } = require('@tanstack/react-query');

const unref = (id) => {
  if (id && typeof id.unref === 'function') id.unref();
  return id;
};

timeoutManager.setTimeoutProvider({
  setTimeout: (cb, ms) => unref(setTimeout(cb, ms)),
  clearTimeout: (id) => clearTimeout(id),
  setInterval: (cb, ms) => unref(setInterval(cb, ms)),
  clearInterval: (id) => clearInterval(id),
});
