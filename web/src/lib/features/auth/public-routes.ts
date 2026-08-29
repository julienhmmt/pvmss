/** Public paths that do not require an authenticated session. */
const PUBLIC_PATHS: ReadonlySet<string> = new Set(['/', '/login']);

/** Public path prefixes (e.g. documentation, about). */
const PUBLIC_PREFIXES: readonly string[] = ['/docs', '/about'];

/** Returns whether the given pathname is a public (unauthenticated) route. */
export function isPublicPath(path: string): boolean {
	if (PUBLIC_PATHS.has(path)) return true;
	return PUBLIC_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}
