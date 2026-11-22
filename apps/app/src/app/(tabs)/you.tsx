import { ApiVersion } from '@/components/api-version';
import { EmptyScreen } from '@/components/empty-screen';

export default function You() {
  return (
    <EmptyScreen titleKey="tabs.you" emptyKey="you.empty">
      <ApiVersion />
    </EmptyScreen>
  );
}
