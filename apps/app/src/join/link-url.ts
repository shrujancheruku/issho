import * as Linking from 'expo-linking';

/**
 * The address a member shares for a join link. On a development build it is the
 * app's own scheme (`issho://join/<code>`), which opens the installed app; on
 * the web it is the page's own origin. The real domain's universal link and
 * app link, which open the app from a message without a scheme, come with the
 * release milestone (they need the domain's association files).
 */
export const joinLinkUrl = (code: string) => Linking.createURL(`join/${code}`);
