// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { focusManager, QueryClient, QueryObserver } from '@tanstack/react-query';
import { AppState } from 'react-native';

import { bindFocusToAppState } from '../app-focus';

test('returning to the app refetches a stale query on screen, and going to the background does not', async () => {
  let emit: (s: string) => void = () => {};
  const remove = jest.fn();
  jest.spyOn(AppState, 'addEventListener').mockImplementation(((_: string, cb: (s: string) => void) => {
    emit = cb;
    return { remove };
  }) as never);

  const qc = new QueryClient();
  qc.mount(); // QueryClientProvider does this: it subscribes the cache to focusManager
  const fetcher = jest.fn(async () => 'x');
  // An observer is a mounted screen; staleTime 0 stands for "older than 30 s".
  const observer = new QueryObserver(qc, { queryKey: ['t'], queryFn: fetcher, staleTime: 0 });
  const unobserve = observer.subscribe(() => {});
  await new Promise((r) => setTimeout(r, 0)); // let the first fetch settle
  const stop = bindFocusToAppState();
  fetcher.mockClear();

  emit('background');
  expect(focusManager.isFocused()).toBe(false);
  expect(fetcher).not.toHaveBeenCalled();

  emit('active');
  await new Promise((r) => setTimeout(r, 0));
  expect(fetcher).toHaveBeenCalledTimes(1);

  stop();
  expect(remove).toHaveBeenCalled();
  unobserve();
  qc.unmount();
});
