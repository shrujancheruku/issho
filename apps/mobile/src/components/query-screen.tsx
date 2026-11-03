import { Code, ConnectError } from '@issho/api-client';
import type { ReactNode } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, View } from 'react-native';

import { ErrorLine, refreshFailedText } from '@/components/error-line';

import { ScreenFrame, type FrameProps } from '@/components/screen-frame';
import { TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useI18n } from '@/i18n/i18n-provider';

/** What a screen needs from a React Query result; any `useQuery` result fits. */
export type QueryLike = {
  isError: boolean;
  /** The last refetch failed (React Query keeps the data from before). Optional: absent means false. */
  isRefetchError?: boolean;
  error: unknown;
  data: unknown;
  refetch: () => unknown;
};

type Loaded<Q extends readonly QueryLike[]> = { [K in keyof Q]: NonNullable<Q[K]['data']> };

const isNotFound = (error: unknown) => error instanceof ConnectError && error.code === Code.NotFound;

/** "Couldn't load" with a retry link, for anything that failed to load, query or not. */
export function LoadFailed({ onRetry, center }: { onRetry: () => void; center?: boolean }) {
  const { t } = useI18n();
  return (
    <>
      <ThemedText themeColor="textSecondary" style={center ? styles.center : undefined}>
        {t('common.loadFailed')}
      </ThemedText>
      <TextLink label={t('common.retry')} onPress={onRetry} center={center} />
    </>
  );
}

/**
 * A small "Couldn't refresh" line for when the data on screen is older than
 * it should be because the last refetch failed; nothing otherwise, and gone
 * on the next success. The rule for every screen that has its data: keep
 * showing it, and say so here, rather than swap it for the failed state.
 */
export function RefreshFailed({ queries }: { queries: readonly QueryLike[] }) {
  const failed = queries.find((q) => q.isRefetchError);
  return failed ? (
    <View style={styles.refresh}>
      <ErrorLine type="small" error={failed.error} text={refreshFailedText} />
    </View>
  ) : null;
}

/**
 * What to show while `queries` haven't all loaded: `notFound` if one of
 * them says the thing doesn't exist, a retry (of every failed query) if
 * one failed, otherwise a spinner. Renders no layout of its own; the
 * caller places it.
 */
export function LoadState({ queries, notFound, center }: { queries: readonly QueryLike[]; notFound?: string; center?: boolean }) {
  const failed = queries.filter((q) => q.isError);
  if (notFound && failed.some((q) => isNotFound(q.error))) {
    return <ThemedText themeColor="textSecondary">{notFound}</ThemedText>;
  }
  if (failed.length) return <LoadFailed center={center} onRetry={() => failed.forEach((q) => void q.refetch())} />;
  return <ActivityIndicator />;
}

/**
 * A screen that loads data: its frame with the not-found / failed /
 * spinner state until every query has data, then `children` with the
 * data, in the same order as `queries`. Having data is what counts, not
 * the last fetch: when a refetch fails the data stays, with a "Couldn't
 * refresh" line above it (the failed state is only for a screen with no
 * data yet). `notFound` likewise applies only while a query has no data: a
 * refetch that fails with NotFound keeps the stale body. `children` renders the body (and
 * its own scrolling), because the success layouts differ screen to screen.
 */
export function QueryScreen<const Q extends readonly QueryLike[]>({
  queries,
  notFound,
  children,
  ...frame
}: FrameProps & {
  queries: Q;
  /** Shown when a query's error is "not found". */
  notFound?: string;
  children: (data: Loaded<Q>) => ReactNode;
}) {
  return (
    <ScreenFrame {...frame}>
      {queries.every((q) => q.data !== undefined) ? (
        <>
          <RefreshFailed queries={queries} />
          {children(queries.map((q) => q.data) as Loaded<Q>)}
        </>
      ) : (
        <ScrollView contentContainerStyle={styles.state}>
          <LoadState queries={queries} notFound={notFound} />
        </ScrollView>
      )}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  state: { padding: Spacing.three, paddingTop: Spacing.two, gap: Spacing.three },
  refresh: { paddingHorizontal: Spacing.three, paddingBottom: Spacing.two },
  center: { textAlign: 'center' },
});
