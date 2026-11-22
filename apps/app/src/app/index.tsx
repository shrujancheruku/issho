import { Redirect } from 'expo-router';

/**
 * Plans opens first for a visitor with no account (docs/spec/app-shape.md,
 * "What opens first"). Together takes over for a person with something in play
 * once sign-in exists.
 */
export default function Index() {
  return <Redirect href="/plans" />;
}
