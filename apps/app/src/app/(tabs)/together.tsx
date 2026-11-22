import { EmptyScreen } from '@/components/empty-screen';
import { RequireSignIn } from '@/components/require-sign-in';

export default function Together() {
  return (
    <RequireSignIn path="/together">
      <EmptyScreen titleKey="tabs.together" emptyKey="together.empty" />
    </RequireSignIn>
  );
}
