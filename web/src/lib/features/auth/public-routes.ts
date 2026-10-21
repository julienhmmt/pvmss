/** Public paths that do not require an authenticated session. */
const PUBLIC_PATHS: ReadonlySet<string> = new Set(['/', '/login']);

/** Public path prefixes (e.g. documentation, about). */
const PUBLIC_PREFIXES: readonly string[] = ['/docs', '/about'];

/** Returns whether the given pathname is a public (unauthenticated) route. */
export function isPublicPath(path: string): boolean {
	if (PUBLIC_PATHS.has(path)) return true;
	return PUBLIC_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}

const NEXT_BASE = 'http://pvmss.invalid';

/** The in-app path to return to after sign-in, or null when `raw` is missing,
 *  leaves the app (`//host`, `/\host`, a scheme) or points back at /login.
 *  Parsing with URL catches what a browser would also resolve off-site. */
export function safeNext(raw: string | null): string | null {
	if (!raw?.startsWith('/')) return null;
	let url: URL;
	try {
		url = new URL(raw, NEXT_BASE);
	} catch {
		return null;
	}
	if (url.origin !== NEXT_BASE || url.pathname === '/login') return null;
	return url.pathname + url.search + url.hash;
}

/** The sign-in URL that brings the user back to `path` (pathname + search). */
export function loginHref(path: string): string {
	return `/login?next=${encodeURIComponent(path)}`;
}
