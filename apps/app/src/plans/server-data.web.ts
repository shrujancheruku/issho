import { useLoaderData } from 'expo-router';

/** Data the Expo server fetched for this route while rendering it (the route's `loader`), already in the HTML a client with no JavaScript gets. */
export function useServerData<T>(): T | undefined {
  return useLoaderData() as T | undefined;
}
