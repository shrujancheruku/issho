// Exists only as a web module: Metro resolves `./web-only` to this file on the
// web and to nothing on iOS and Android, which is the failure the native bundle
// check must catch (scripts/native-bundle.test.sh copies a route that imports it).
export const webOnly = 'web';
