import { useEffect, useState } from 'react';
import { StyleSheet, Text } from 'react-native';

import { useApi } from '@/api/api-provider';
import { FontSize } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * The signed-in person's own account, from the API. The first call makes the
 * account (to join is to make one), in the language the person is reading in.
 * Shows nothing until it answers.
 */
export function Account() {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const [email, setEmail] = useState<string | null | undefined>(undefined);

  useEffect(() => {
    let live = true;
    api.account
      .getAccount({ displayLanguage: lang })
      .then((res) => live && setEmail(res.account?.email ?? null))
      .catch(() => live && setEmail(null));
    return () => {
      live = false;
    };
    // The language only matters to the call that makes the account, so a later change does not call again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api]);

  if (email === undefined) return null;
  return (
    <Text testID="account" style={[styles.text, { color: theme.textSecondary }]}>
      {email === null ? t('you.accountError') : t('you.email', { email })}
    </Text>
  );
}

const styles = StyleSheet.create({
  text: { fontSize: FontSize.default },
});
