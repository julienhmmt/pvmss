/** Public paths that do not require an authenticated session. */
const PUBLIC_PATHS: readonly string[] = ['/', '/login'];

/** Public path prefixes (e.g. documentation). */
const PUBLIC_PREFIXES: readonly string[] = ['/docs'];

/** Returns whether the given pathname is a public (unauthenticated) route. */
export function isPublicPath(path: string): boolean {
	if (PUBLIC_PATHS.includes(path)) return true;
	return PUBLIC_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}
