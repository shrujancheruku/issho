// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

module.exports = defineConfig([
  expoConfig,
  {
    ignores: ['dist/*'],
  },
  {
    // Hermes (the engine on phones, including Expo Go) lacks these newer
    // array methods, while Node (the tests) and browsers have them, so a call
    // only fails on a device: toSorted() crashed signing up on Android.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/__tests__/**'],
    rules: {
      'no-restricted-syntax': [
        'error',
        ...['toSorted', 'toReversed', 'toSpliced', 'with', 'findLast', 'findLastIndex'].map((name) => ({
          selector: `CallExpression[callee.property.name='${name}']`,
          message: `${name}() isn't available in Hermes on every device; copy and use the mutating method instead.`,
        })),
      ],
    },
  },
]);
