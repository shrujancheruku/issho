import { execFileSync } from 'node:child_process';
import path from 'node:path';

// "Lint fails on a string literal in a screen file": the app's real eslint
// config run on a snippet, as if it were a file at `filePath`.
function messages(code: string, filePath: string): string[] {
  const out = execFileSync(process.execPath, [path.join(__dirname, '../../scripts/lint-snippet.js'), filePath], {
    input: code,
    cwd: path.join(__dirname, '../..'),
  }).toString();
  return (JSON.parse(out) as { ruleId: string | null; message: string }[])
    .filter((m) => m.ruleId === 'no-restricted-syntax')
    .map((m) => m.message);
}

const screen = (body: string) => `import { Text } from 'react-native';\nexport default function S() { return ${body}; }\n`;

describe('string literals in screen files', () => {
  it('fails on text written in JSX', () => {
    expect(messages(screen('<Text>Plans</Text>'), 'src/app/(tabs)/plans.tsx')).toHaveLength(1);
  });

  it('fails on Vietnamese text', () => {
    expect(messages(screen('<Text>Kế hoạch</Text>'), 'src/app/(tabs)/plans.tsx')).toHaveLength(1);
  });

  it('fails on a string in braces and on a label attribute', () => {
    expect(messages(screen("<Text>{'Plans'}</Text>"), 'src/app/(tabs)/plans.tsx')).toHaveLength(1);
    expect(messages(screen('<Text accessibilityLabel="Plans" />'), 'src/app/(tabs)/plans.tsx')).toHaveLength(1);
  });

  it('fails in a shared component too', () => {
    expect(messages(screen('<Text>Plans</Text>'), 'src/components/empty.tsx')).toHaveLength(1);
  });

  it('passes text that comes through t()', () => {
    expect(messages(screen("<Text testID=\"x\">{t('plans.empty')}</Text>"), 'src/app/(tabs)/plans.tsx')).toEqual([]);
  });
});
