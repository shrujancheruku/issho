import { StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * Shown in place of a map that can't be drawn here: no maps module or key in
 * this build (phones), or no web key (browsers). Kept apart from native-maps
 * so the web maps can use it without loading react-native-maps.
 */
export function MapUnavailable() {
  const { t } = useI18n();
  return (
    <View style={styles.missing}>
      <ThemedText type="small" themeColor="textSecondary">
        {t('map.unavailable')}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  missing: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', padding: 24 },
});
