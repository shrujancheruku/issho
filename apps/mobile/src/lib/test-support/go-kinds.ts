// The app has no Node types; jest runs in Node, so declare the few things used.
declare const __dirname: string;
declare function require(id: string): any;
const fs = require('fs') as {
  readdirSync(path: string, o: { withFileTypes: true }): { name: string; isDirectory(): boolean }[];
  readFileSync(path: string, encoding: string): string;
};

const root = `${__dirname}/../../../../../backend/internal`;

function goFiles(dir: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    if (e.isDirectory()) return goFiles(`${dir}/${e.name}`);
    return e.name.endsWith('.go') && !e.name.endsWith('_test.go') ? [`${dir}/${e.name}`] : [];
  });
}

/**
 * The backend's notification kinds: every `Notify* = "kind"` constant in a
 * non-test Go file under backend/internal. Shared by the push tests so they
 * can't drift apart.
 */
export const goKinds: string[] = goFiles(root).flatMap((f) =>
  [...fs.readFileSync(f, 'utf8').matchAll(/^\s*Notify\w+\s*=\s*"([a-z0-9_]+)"/gm)].map((m) => m[1]),
);
