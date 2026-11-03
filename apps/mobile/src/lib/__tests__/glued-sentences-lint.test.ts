/// <reference types="jest" />
// @jest-environment node
import { Linter } from 'eslint';

// The lint rule that keeps sentences whole in the catalogue (eslint.config.js,
// apps/mobile/README.md "Writing UI text"): each reintroduced example must be
// flagged, and the template form must pass.
// A made-up project root: the config only matches paths against it.
const root = '/virtual';

// The Linter takes the flat config directly (the ESLint class would load the
// config file with a dynamic import, which Jest's sandbox refuses).
// eslint-disable-next-line @typescript-eslint/no-require-imports
const config = require('../../../eslint.config.js');

const idOf = (m: { message: string }) => /^\[([a-z-]+)\]/.exec(m.message)?.[1];

async function messages(code: string) {
  const linter = new Linter({ cwd: root, configType: 'flat' });
  const found = linter.verify(code, config, { filename: `${root}/src/app/example.tsx` });
  return found.filter((m) => m.ruleId === 'no-restricted-syntax' && /catalogue template/.test(m.message));
}

const wrap = (jsx: string) => `export const X = () => { const { t } = useI18n(); return (<Text>${jsx}</Text>); };`;

describe('glued sentences are flagged', () => {
  // Each example names the selector (its id is in the message) that must fire, and only that one.
  const bad: Record<string, [string, string]> = {
    'text after t()': ['text-after-t', wrap("{t('plan.signupBy')}: {date}")],
    'full width punctuation': ['text-after-t', wrap("{t('plan.signupBy')}：{date}")],
    'full stop before t()': ['text-before-t', wrap("{area}. {t('plan.interested', { count })}")],
    'two t() calls side by side': ['t-beside-t', wrap("{t('a')} {t('b')}")],
    'template literal': ['template-literal', "const s = (t) => `${t('a')}: ${x}`;"],
    plus: ['plus', "const s = (t) => t('a') + ': ' + x;"],
    join: ['join', "const s = (t) => [t('a'), t('b')].join('. ');"],
    'template with leading punctuation after t()': [
      'punctuation-after-t',
      wrap("{t('plan.areaDistance')}\n{language ? `. ${language}` : ''}"),
    ],
    'string with leading punctuation after t()': ['punctuation-after-t', wrap("{t('a')}\n{x ? '. y' : ''}")],
  };
  for (const [name, [id, code]] of Object.entries(bad)) {
    it(name, async () => {
      expect((await messages(code)).map(idOf)).toEqual([id]);
    });
  }

  it('lets a whole template and plain markup through', async () => {
    const ok = [
      wrap("{t('plan.signupByDate', { date })}"),
      wrap("{t('a')}\n{t('b')}"),
      wrap("<B>{t('a')}</B>"),
      "const s = (t) => t('plan.slot', { day: 'x', block: t('b') });",
    ];
    for (const code of ok) expect(await messages(code)).toEqual([]);
  });
});
