// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

// No string is written in a screen file: the app's words live in
// src/i18n/*.json and reach a screen through t() (docs/spec/quality.md, "Bars
// that aren't tests"). Screens and the components they share are both screen
// files here, or a screen could hand its text to a component.
const WORDS = '[value=/\\p{L}/u]';
const MESSAGE = 'Write words in src/i18n/en.json and vi.json and use t("key"): no string literal in a screen file.';
const SCREEN_TEXT = [
  { selector: `JSXText${WORDS}`, message: MESSAGE },
  { selector: `JSXExpressionContainer > Literal${WORDS}`, message: MESSAGE },
  { selector: `JSXExpressionContainer > TemplateLiteral > TemplateElement[value.raw=/\\p{L}/u]`, message: MESSAGE },
  {
    selector: `JSXAttribute[name.name=/^(title|label|placeholder|alt|accessibilityLabel|accessibilityHint|aria-label)$/] > Literal`,
    message: MESSAGE,
  },
  { selector: `Property[key.name=/^(title|tabBarLabel|headerTitle)$/] > Literal${WORDS}`, message: MESSAGE },
];

module.exports = defineConfig([
  expoConfig,
  { ignores: ['dist/*', '.expo/*'] },
  {
    files: ['src/app/**/*.{ts,tsx}', 'src/components/**/*.{ts,tsx}'],
    ignores: ['**/__tests__/**'],
    rules: { 'no-restricted-syntax': ['error', ...SCREEN_TEXT] },
  },
]);
