import { JoinLinkProblemReason, JoinLinkState, type JoinLink } from '@issho/api-client';
import { useCallback, useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { useApi } from '@/api/api-provider';
import { Button, ErrorText } from '@/components/form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { joinLinkUrl } from '@/join/link-url';
import { joinLinkReason } from '@/join/problems';
import { shareJoinLink } from '@/join/share';
import { formatDate } from '@/lib/format';
import type { StringKey } from '@/i18n/strings';

/** The Pilot's one city's clock, for the day a link expires; a member's own city comes with the You tab's settings. */
const CITY_TIME_ZONE = 'Asia/Ho_Chi_Minh';

type Loaded = { status: 'loading' } | { status: 'failed' } | { status: 'ready'; links: JoinLink[]; remaining: number };

/**
 * The member's own join links (docs/spec/people.md, "Join links"): make one,
 * share it any way they like, cancel one that is still outstanding. Issho sends
 * nothing. A link that was used says only that; never by whom.
 */
export function JoinLinks() {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const [loaded, setLoaded] = useState<Loaded>({ status: 'loading' });
  const [notice, setNotice] = useState<{ key: StringKey; bad: boolean } | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    () =>
      api.joinLink.listMyJoinLinks({}).then(
        (res) => setLoaded({ status: 'ready', links: res.links, remaining: res.remaining }),
        () => setLoaded({ status: 'failed' }),
      ),
    [api],
  );
  useEffect(() => {
    void load();
  }, [load]);

  const make = async () => {
    setBusy(true);
    setNotice(null);
    try {
      await api.joinLink.createJoinLink({});
      await load();
    } catch (err) {
      const reason = joinLinkReason(err);
      setNotice({
        key: reason === JoinLinkProblemReason.NONE_LEFT ? 'joinLinks.noneLeft' : reason === JoinLinkProblemReason.NOT_ADMITTED ? 'joinLinks.notAdmitted' : 'joinLinks.makeFailed',
        bad: true,
      });
    } finally {
      setBusy(false);
    }
  };

  const cancel = async (id: string) => {
    setBusy(true);
    setNotice(null);
    try {
      await api.joinLink.cancelJoinLink({ id });
    } catch {
      setNotice({ key: 'joinLinks.cancelFailed', bad: true });
    }
    // Whatever the answer, the list is what the API says now (a link may have been used meanwhile).
    await load();
    setBusy(false);
  };

  const share = async (code: string) => {
    setNotice(null);
    const url = joinLinkUrl(code);
    const result = await shareJoinLink(t('joinLinks.shareMessage', { url }), url);
    if (result === 'copied') setNotice({ key: 'joinLinks.copied', bad: false });
    if (result === 'failed') setNotice({ key: 'joinLinks.shareFailed', bad: true });
  };

  const stateText = (link: JoinLink) => {
    if (link.state === JoinLinkState.USED) return t('joinLinks.used');
    if (link.state === JoinLinkState.EXPIRED) return t('joinLinks.expired');
    const expires = link.expiresAt ? formatDate(new Date(Number(link.expiresAt.seconds) * 1000), { timeZone: CITY_TIME_ZONE, lang }) : '';
    return t('joinLinks.outstanding', { date: expires });
  };

  return (
    <View testID="join-links" style={styles.block}>
      <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
        {t('joinLinks.title')}
      </Text>
      <Text style={[styles.body, { color: theme.textSecondary }]}>{t('joinLinks.intro')}</Text>
      {loaded.status === 'failed' ? (
        <>
          <ErrorText>{t('joinLinks.loadFailed')}</ErrorText>
          <Button quiet label={t('join.retry')} onPress={load} />
        </>
      ) : null}
      {loaded.status === 'ready' ? (
        <>
          {loaded.links.map((link) => (
            <View key={link.id} style={[styles.row, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }]}>
              <Text style={[styles.body, { color: theme.text }]}>{stateText(link)}</Text>
              {link.state === JoinLinkState.OUTSTANDING ? (
                <View style={styles.actions}>
                  <Button label={t('joinLinks.share')} onPress={() => share(link.code)} />
                  <Button quiet label={t('joinLinks.cancel')} onPress={() => cancel(link.id)} disabled={busy} />
                </View>
              ) : null}
            </View>
          ))}
          <Text style={[styles.body, { color: theme.textSecondary }]}>
            {loaded.remaining > 0 ? t('joinLinks.remaining', { count: loaded.remaining }) : t('joinLinks.noneLeft')}
          </Text>
          {loaded.remaining > 0 ? <Button label={t('joinLinks.make')} onPress={make} disabled={busy} /> : null}
        </>
      ) : null}
      {notice ? notice.bad ? <ErrorText>{t(notice.key)}</ErrorText> : <Text accessibilityRole="alert" style={[styles.body, { color: theme.ok }]}>{t(notice.key)}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  block: { gap: Spacing.two },
  heading: { fontSize: FontSize.default, fontWeight: '700' },
  body: { fontSize: FontSize.default },
  row: { borderWidth: 1, borderRadius: 8, padding: Spacing.three, gap: Spacing.two },
  actions: { gap: Spacing.one },
});
