// noVNC touches `window` at import time; render this page in the browser only
// (dev SSR crashed with "window is not defined" when opened directly).
export const ssr = false;
