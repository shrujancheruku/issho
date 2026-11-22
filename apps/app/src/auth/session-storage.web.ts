/**
 * The web keeps the session in the browser's own storage, which Supabase uses
 * when it is given none. Not importing AsyncStorage here keeps the server render
 * (which has no storage and holds no session) and the web bundle free of it.
 */
export const sessionStorage = undefined;
