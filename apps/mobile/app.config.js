// Extends app.json with values that can't be committed (the repo is public).
//
// google-services.json (Firebase, for Android push via FCM) is gitignored.
// Locally it's apps/mobile/google-services.json; on EAS Build it's the
// GOOGLE_SERVICES_JSON file environment variable, which EAS writes to a
// temporary path. Without either, the app builds without push on Android
// (Expo Go, the web, CI).
// See README.md, "Push notifications".
const fs = require('node:fs');
const path = require('node:path');

/** @param {import('expo/config').ConfigContext} ctx @returns {import('expo/config').ExpoConfig} */
module.exports = ({ config }) => {
  const local = path.join(__dirname, 'google-services.json');
  const googleServicesFile = process.env.GOOGLE_SERVICES_JSON ?? (fs.existsSync(local) ? './google-services.json' : undefined);
  return {
    ...config,
    android: { ...config.android, ...(googleServicesFile && { googleServicesFile }) },
  };
};
