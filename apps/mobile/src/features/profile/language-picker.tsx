import { useMemo, useState } from 'react';
import { FlatList, Modal, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import { useLanguages } from './data';

/** Full-screen searchable list of every language (CatalogService.ListLanguages). */
export function LanguagePicker({
  visible,
  exclude = [],
  onPick,
  onClose,
}: {
  visible: boolean;
  exclude?: string[];
  onPick: (code: string) => void;
  onClose: () => void;
}) {
  const theme = useTheme();
  const { t } = useI18n();
  const languages = useLanguages();
  const [query, setQuery] = useState('');

  const items = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (languages.data ?? []).filter(
      (l) =>
        !exclude.includes(l.code) &&
        (!q || l.code === q || l.autonym.toLowerCase().includes(q) || l.name.toLowerCase().includes(q)),
    );
  }, [languages.data, exclude, query]);

  return (
    <Modal visible={visible} animationType="slide" presentationStyle="pageSheet" onRequestClose={onClose}>
      <ThemedView style={styles.fill}>
        <SafeAreaView style={styles.fill} edges={['top', 'bottom']}>
          <View style={styles.header}>
            <ThemedText type="smallBold">{t('language.pick')}</ThemedText>
            <Pressable accessibilityRole="button" onPress={onClose} hitSlop={8}>
              <ThemedText type="link">{t('common.cancel')}</ThemedText>
            </Pressable>
          </View>
          <TextInput
            style={[styles.search, { color: theme.text, backgroundColor: theme.backgroundElement }]}
            value={query}
            onChangeText={setQuery}
            placeholder={t('common.search')}
            placeholderTextColor={theme.textSecondary}
            autoCorrect={false}
            autoCapitalize="none"
          />
          {languages.isError ? (
            <Pressable onPress={() => void languages.refetch()} style={styles.row}>
              <ThemedText>{t('common.loadFailed')}</ThemedText>
              <ThemedText type="link">{t('common.retry')}</ThemedText>
            </Pressable>
          ) : (
            <FlatList
              data={items}
              keyExtractor={(l) => l.code}
              keyboardShouldPersistTaps="handled"
              renderItem={({ item }) => (
                <Pressable
                  accessibilityRole="button"
                  onPress={() => {
                    setQuery('');
                    onPick(item.code);
                  }}
                  style={({ pressed }) => [styles.row, pressed && { backgroundColor: theme.backgroundSelected }]}>
                  <ThemedText>{item.autonym}</ThemedText>
                  {item.name !== item.autonym && (
                    <ThemedText type="small" themeColor="textSecondary">
                      {item.name}
                    </ThemedText>
                  )}
                </Pressable>
              )}
            />
          )}
        </SafeAreaView>
      </ThemedView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  header: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', padding: Spacing.three },
  search: { marginHorizontal: Spacing.three, borderRadius: 12, padding: Spacing.three, fontSize: 17 },
  row: { paddingVertical: Spacing.three, paddingHorizontal: Spacing.four, gap: Spacing.half },
});
