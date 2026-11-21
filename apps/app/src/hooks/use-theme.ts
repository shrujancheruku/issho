import { Colors } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';

/** The colours of the theme being drawn. */
export function useTheme() {
  return Colors[useColorScheme()];
}
