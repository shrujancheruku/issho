// Expo's Metro config, plus Sentry's: it stamps each bundle with a debug ID,
// so Sentry matches crash reports to the source maps EAS builds upload
// (the @sentry/react-native/expo plugin in app.json; docs in README.md).
const { getSentryExpoConfig } = require('@sentry/react-native/metro');

module.exports = getSentryExpoConfig(__dirname);
