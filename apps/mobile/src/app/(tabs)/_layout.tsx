import { router } from 'expo-router';
import { useCallback, useEffect } from 'react';

import AppTabs from '@/components/app-tabs';
import { registerForPush, useNotificationTaps } from '@/lib/push';

// The main app, available once the user is signed in and onboarded (see app/_layout.tsx).
export default function TabsLayout() {
  // Keeps the device registered without asking: permission is asked for
  // when it's worth it (signing up for a plan).
  useEffect(() => {
    void registerForPush({ ask: false });
  }, []);
  useNotificationTaps(useCallback((id: string) => router.push({ pathname: '/event/[id]', params: { id } }), []));
  return <AppTabs />;
}
