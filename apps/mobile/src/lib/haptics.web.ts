// Browsers have no haptics worth using (the Vibration API buzzes); the web stays silent.
const none = () => {};
export const haptics = { selection: none, tap: none, success: none };
