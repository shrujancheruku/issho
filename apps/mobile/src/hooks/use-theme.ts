/**
 * The colours of the theme being drawn (the person's choice or the
 * system's; see hooks/use-color-scheme).
 */

import { Colors } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';

export function useTheme() {
  return Colors[useColorScheme()];
}
