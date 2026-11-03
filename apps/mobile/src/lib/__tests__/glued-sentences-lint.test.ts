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

async function messages(code: string) {
  const linter = new Linter({ cwd: root, configType: 'flat' });
  const found = linter.verify(code, config, { filename: `${root}/src/app/example.tsx` });
  return found.filter((m) => m.ruleId === 'no-restricted-syntax' && /catalogue template/.test(m.message));
}

const wrap = (jsx: string) => `export const X = () => { const { t } = useI18n(); return (<Text>${jsx}</Text>); };`;

describe('glued sentences are flagged', () => {
  const bad: Record<string, string> = {
    'text after t()': wrap("{t('plan.signupBy')}: {date}"),
    'full stop between pieces': wrap("{area}. {t('plan.interested', { count })}"),
    'full width punctuation': wrap("{t('plan.signupBy')}：{date}"),
    'two t() calls side by side': wrap("{t('a')} {t('b')}"),
    'template literal': "const s = (t) => `${t('a')}: ${x}`;",
    plus: "const s = (t) => t('a') + ': ' + x;",
    join: "const s = (t) => [t('a'), t('b')].join('. ');",
  };
  for (const [name, code] of Object.entries(bad)) {
    it(name, async () => {
      expect((await messages(code)).length).toBeGreaterThan(0);
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
