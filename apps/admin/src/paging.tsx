import { ConnectError } from '@issho/api-client';
import { useInfiniteQuery } from '@tanstack/react-query';

/**
 * A list that loads a page at a time (AIP-158: the response's
 * nextPageToken is the next request's pageToken, empty on the last page).
 * The tokens belong to one request, so they are not in the query key: a
 * change of filter is a new key and starts from the first page.
 */
export function usePaged<R extends { nextPageToken: string }>(queryKey: readonly unknown[], fetchPage: (pageToken: string) => Promise<R>, enabled = true) {
  return useInfiniteQuery({
    queryKey,
    enabled,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextPageToken || undefined,
  });
}

/** Every row loaded so far, in the order sent. */
export function rowsOf<R, T>(data: { pages: R[] } | undefined, pick: (page: R) => T[]): T[] {
  return data?.pages.flatMap(pick) ?? [];
}

type More = { hasNextPage: boolean; isFetchingNextPage: boolean; isFetchNextPageError: boolean; error: unknown; fetchNextPage: () => unknown };

/** "Load more" under a list; nothing on the last page. */
export function LoadMore({ query }: { query: More }) {
  if (!query.hasNextPage && !query.isFetchNextPageError) return null;
  return (
    <div className="row pager">
      <button className="secondary" disabled={query.isFetchingNextPage} onClick={() => { void query.fetchNextPage(); }}>
        {query.isFetchingNextPage ? 'Loading…' : 'Load more'}
      </button>
      {query.isFetchNextPageError && <span className="error">{ConnectError.from(query.error).message}</span>}
    </div>
  );
}
