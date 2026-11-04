// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

// A sentence built from t() pieces and literal punctuation can't be reordered
// or punctuated by a translator (Japanese uses "：" and "。"). It must be one
// catalogue template with placeholders (apps/mobile/README.md, "Writing UI text").
const T = "CallExpression[callee.name='t']";
const PUNCT = String.raw`(:|\.|,|;|!|\?|·|–|—|\||\)|：|。|、)`;
const glued = (id, selector) => ({
  selector,
  message:
    '[' + id + '] Don\'t glue t() output with punctuation or other t() output: make one catalogue template with placeholders so translators can reorder and punctuate it (apps/mobile/README.md, "Writing UI text").',
});

// Text sizes and colours come from the theme, so a restyle (or a dark mode
// fix) is one edit (docs/design-system.md, apps/mobile/README.md "Theme").
// A fontSize written as a number (or computed, or from anywhere but FontSize)
// is a size no ThemedText type has; FontSize.default (a TextInput's size) is
// fine. A quoted key ('fontSize') counts too.
const FONT_SIZE = {
  selector:
    "Property:matches([key.name='fontSize'], [key.value='fontSize']):not([value.type='MemberExpression'][value.object.name='FontSize'])",
  message:
    '[font-size] Use a ThemedText type (caption, small, default, heading, subtitle, title) instead of an inline fontSize. A deliberate display size (a map pin, a code field) needs an eslint-disable comment that says why (apps/mobile/README.md, "Theme").',
};
const COLOUR_MESSAGE =
  '[colour-literal] Use a colour from useTheme() or Brand instead of a literal: add a token to constants/theme.ts with a light and a dark value (apps/mobile/README.md, "Theme").';
const COLOUR = [
  { selector: 'Literal[value=/^#[0-9a-fA-F]{3,8}$/]', message: COLOUR_MESSAGE },
  { selector: 'Literal[value=/^(rgba?|hsla?)\\(/]', message: COLOUR_MESSAGE },
  { selector: 'TemplateElement[value.raw=/(rgba?|hsla?)\\(/]', message: COLOUR_MESSAGE },
  // Named colours only as a style value, so ordinary strings called "white" pass; 'transparent' stays allowed.
  { selector: 'Property > Literal.value[value=/^(white|black)$/i]', message: COLOUR_MESSAGE },
];

// Hermes (the engine on phones, including Expo Go) lacks these newer
// array methods, while Node (the tests) and browsers have them, so a call
// only fails on a device: toSorted() crashed signing up on Android.
const BASE = [
  ...['toSorted', 'toReversed', 'toSpliced', 'with', 'findLast', 'findLastIndex'].map((name) => ({
    selector: `CallExpression[callee.property.name='${name}']`,
    message: `${name}() isn't available in Hermes on every device; copy and use the mutating method instead.`,
  })),
  glued('text-after-t', String.raw`JSXExpressionContainer:has(> ${T}) + JSXText[value=/^\s*${PUNCT}/]`),
  glued('text-before-t', String.raw`JSXText[value=/(${PUNCT}|\()\s*$/] + JSXExpressionContainer:has(> ${T})`),
  glued('t-beside-t', String.raw`JSXExpressionContainer:has(> ${T}) + JSXText[value=/^ +$/] + JSXExpressionContainer:has(> ${T})`),
  glued('template-literal', `TemplateLiteral:has(> ${T}):has(> TemplateElement[value.raw=/./])`),
  glued('plus', `BinaryExpression[operator='+']:has(> ${T})`),
  glued('join', `CallExpression[callee.property.name='join'][callee.object.type='ArrayExpression']:not([arguments.0.value=/^\\s*$/]):has(${T})`),
  glued('punctuation-after-t', String.raw`JSXExpressionContainer:has(> ${T}) ~ JSXExpressionContainer:has(TemplateElement[value.raw=/^${PUNCT}/], Literal[value=/^${PUNCT}/])`),
  // Left and right don't mirror in right-to-left languages; start and
  // end do. (Absolute left/right positions and map padding are allowed.)
  {
    selector:
      'Property[key.name=/^(margin|padding)(Left|Right)$|^border(Left|Right)(Width|Color|Style)$|^border(Top|Bottom)(Left|Right)Radius$/]',
    message: 'Use the Start/End form of this style so layouts mirror in right-to-left languages.',
  },
  {
    selector: "Property[key.name='textAlign'] Literal[value=/^(left|right)$/]",
    message: "textAlign left/right doesn't mirror; leave it unset (it follows the writing direction) unless you mean it.",
  },
];

module.exports = defineConfig([
  expoConfig,
  {
    ignores: ['dist/*'],
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/__tests__/**', 'src/constants/theme.ts', 'src/components/themed-text.tsx'],
    rules: { 'no-restricted-syntax': ['error', ...BASE, FONT_SIZE, ...COLOUR] },
  },
  {
    // The type scale lives here, so it may set sizes; colours still come from the theme.
    files: ['src/components/themed-text.tsx'],
    rules: { 'no-restricted-syntax': ['error', ...BASE, ...COLOUR] },
  },
  {
    // The theme is where sizes and colour values are defined.
    files: ['src/constants/theme.ts'],
    rules: { 'no-restricted-syntax': ['error', ...BASE] },
  },
]);
