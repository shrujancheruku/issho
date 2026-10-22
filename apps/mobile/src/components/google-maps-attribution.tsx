import { StyleSheet, Text } from 'react-native';

import { useColorScheme } from '@/hooks/use-color-scheme';

/**
 * The attribution Google requires wherever Places data (venue names,
 * addresses) is shown without a Google map. Google's rules for the text
 * form: exactly "Google Maps", never translated (so not from the i18n
 * catalogue), 12–16sp, and only white, #1F1F1F or #5E5E5E. This uses the
 * smallest size, grey on light backgrounds and white on dark (#5E5E5E is
 * near-invisible on black). Don't restyle it to match the theme: the app's
 * own greys aren't allowed.
 * https://developers.google.com/maps/documentation/places/web-service/policies
 */
export function GoogleMapsAttribution() {
  const dark = useColorScheme() === 'dark';
  return (
    <Text accessibilityLabel="Google Maps" style={[styles.text, { color: dark ? '#FFFFFF' : '#5E5E5E' }]}>
      Google Maps
    </Text>
  );
}

const styles = StyleSheet.create({
  text: { fontSize: 12, lineHeight: 16, alignSelf: 'flex-end' },
});
