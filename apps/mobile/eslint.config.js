// https://docs.expo.dev/guides/using-eslint/
const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

// A sentence built from t() pieces and literal punctuation can't be reordered
// or punctuated by a translator (Japanese uses "：" and "。"). It must be one
// catalogue template with placeholders (apps/mobile/README.md, "Writing UI text").
const T = "CallExpression[callee.name='t']";
const PUNCT = String.raw`(:|\.|,|;|!|\?|·|–|—|\||\)|：|。|、)`;
const glued = (selector) => ({
  selector,
  message:
    'Don\'t glue t() output with punctuation or other t() output: make one catalogue template with placeholders so translators can reorder and punctuate it (apps/mobile/README.md, "Writing UI text").',
});

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
        glued(String.raw`JSXExpressionContainer:has(> ${T}) + JSXText[value=/^\s*${PUNCT}/]`),
        glued(String.raw`JSXText[value=/(${PUNCT}|\()\s*$/] + JSXExpressionContainer:has(> ${T})`),
        glued(String.raw`JSXExpressionContainer:has(> ${T}) + JSXText[value=/^ +$/] + JSXExpressionContainer:has(> ${T})`),
        glued(`TemplateLiteral:has(> ${T}):has(> TemplateElement[value.raw=/./])`),
        glued(`BinaryExpression[operator='+']:has(> ${T})`),
        glued(`CallExpression[callee.property.name='join'][callee.object.type='ArrayExpression']:not([arguments.0.value=/^\\s*$/]):has(${T})`),
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
      ],
    },
  },
]);
