import { useRouter } from 'expo-router';

import { useOpenOnTogether } from '@/signup/use-open-on-together';

/**
 * What the app opens on (docs/spec/app-shape.md, "What opens first"): Together
 * when the person has something in play, Plans for everyone else, a visitor
 * with no account included. On the web the server never gets here: it sends
 * `/` to `/plans?open=1` (`+middleware.ts`), and the Plans screen applies the
 * same rule once it knows who is looking.
 */
export default function Index() {
  const router = useRouter();
  useOpenOnTogether(() => router.replace('/plans'));
  return null;
}
