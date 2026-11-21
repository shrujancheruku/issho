// A route that imports a web-only module. native-bundle.test.sh copies it into
// apps/app/src/app for one run and removes it again; it is never a real route.
import { webOnly } from '../../scripts/fixtures/native-bundle/web-only';

export const fixtureValue = webOnly;

export default function NativeBundleFixture() {
  return null;
}
