/// <reference types="jest" />
// @jest-environment node
import { Linter } from 'eslint';

// The lint bans that keep text sizes and colours in the theme (eslint.config.js,
// apps/mobile/README.md "Theme"): each reintroduced example must be flagged by
// its own rule, and the allowed forms must pass.
const root = '/virtual';

// The Linter takes the flat config directly (see glued-sentences-lint.test.ts).
// eslint-disable-next-line @typescript-eslint/no-require-imports
const config = require('../../../eslint.config.js');

const idOf = (m: { message: string }) => /^\[([a-z-]+)\]/.exec(m.message)?.[1];

function ids(code: string, file = 'src/app/example.tsx') {
  const linter = new Linter({ cwd: root, configType: 'flat' });
  const found = linter.verify(code, config, { filename: `${root}/${file}` });
  return found.filter((m) => m.ruleId === 'no-restricted-syntax').map(idOf);
}

describe('inline font sizes are flagged', () => {
  it('a number', () => {
    expect(ids('export const s = { title: { fontSize: 22, lineHeight: 32 } };')).toEqual(['font-size']);
  });

  it('a computed size', () => {
    expect(ids('export const s = (n: number) => ({ fontSize: Math.round(n * 0.36) });')).toEqual(['font-size']);
  });

  it('in JSX style props', () => {
    expect(ids('export const X = () => <Text style={{ fontSize: 12 }}>a</Text>;')).toEqual(['font-size']);
  });

  it('is allowed with a reasoned disable, from the theme, and in the theme itself', () => {
    expect(ids('// eslint-disable-next-line no-restricted-syntax\nexport const s = { code: { fontSize: 28 } };')).toEqual([]);
    expect(ids('export const s = { input: { fontSize: FontSize.default } };')).toEqual([]);
    expect(ids('export const s = { default: { fontSize: 16 } };', 'src/components/themed-text.tsx')).toEqual([]);
    expect(ids('export const s = { fontSize: 16 };', 'src/constants/theme.ts')).toEqual([]);
  });
});

describe('colour literals are flagged', () => {
  const bad: Record<string, string> = {
    rgba: "export const s = { bar: { backgroundColor: 'rgba(255,255,255,0.14)' } };",
    'rgba with spaces': "export const s = { halo: 'rgba(222, 169, 44, 0.3)' };",
    rgb: "export const s = { a: 'rgb(0, 0, 0)' };",
    'short hex': "export const s = { a: '#fff' };",
    'long hex': "export const s = { borderColor: '#ffffff' };",
    'hex with alpha': "export const s = { a: '#ffffff80' };",
    'template literal': 'export const s = (a: number) => ({ a: `rgba(0,0,0,${a})` });',
    'in a prop': 'export const X = () => <Icon color="#5E5E5E" />;',
  };
  for (const [name, code] of Object.entries(bad)) {
    it(name, () => {
      expect(ids(code)).toEqual(['colour-literal']);
    });
  }

  it('is allowed in the theme, and plain strings pass', () => {
    expect(ids("export const Colors = { text: '#131B40', wash: 'rgba(255,255,255,0.14)' };", 'src/constants/theme.ts')).toEqual([]);
    expect(ids("export const s = { a: 'transparent', b: '#', c: 'see #3 for rgba' };")).toEqual([]);
  });

  it('is not enforced in tests', () => {
    expect(ids("jest.mock('x', () => '#000');", 'src/components/__tests__/example.test.tsx')).toEqual([]);
  });
});
