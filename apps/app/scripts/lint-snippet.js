// Lints one snippet with the app's eslint config, as if it were the file at
// argv[2], and prints the messages as JSON. src/__tests__/screen-lint.test.ts
// runs it in a child process because ESLint can't load inside jest-expo's
// module sandbox.
const { ESLint } = require('eslint');

let code = '';
process.stdin.on('data', (chunk) => (code += chunk));
process.stdin.on('end', async () => {
  const [result] = await new ESLint().lintText(code, { filePath: process.argv[2] });
  process.stdout.write(JSON.stringify(result.messages));
});
