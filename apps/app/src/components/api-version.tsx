import { useEffect, useState } from 'react';
import { StyleSheet, Text } from 'react-native';

import { useApi } from '@/api/api-provider';
import { FontSize } from '@/constants/theme';
import { useI18n } from '@/i18n/i18n-provider';
import { useTheme } from '@/hooks/use-theme';

/**
 * The version the API's public health check reports. It is what the dev deploy's
 * walk looks for on the page. It runs in the browser or on the phone, after the
 * page is up, so the server-rendered HTML holds nothing that depends on the API.
 */
export function ApiVersion() {
  const { t } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const [version, setVersion] = useState<string | null | undefined>(undefined);

  useEffect(() => {
    let live = true;
    api.health
      .check({})
      .then((res) => live && setVersion(res.version))
      .catch(() => live && setVersion(null));
    return () => {
      live = false;
    };
  }, [api]);

  if (version === undefined) return null;
  return (
    <Text testID="api-version" style={[styles.text, { color: theme.textSecondary }]}>
      {version === null ? t('apiVersion.unreachable') : t('apiVersion.value', { version })}
    </Text>
  );
}

const styles = StyleSheet.create({
  text: { fontSize: FontSize.small },
});
