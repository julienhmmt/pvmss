import { resolve } from '$app/paths';

/**
 * Resolves an API path with the app's configured base path.
 *
 * Uses SvelteKit's `resolve()` (the non-deprecated replacement for `base`)
 * with a type cast: `resolve()` is typed to accept only known app route
 * pathnames, but at runtime it simply prepends `base` to any absolute path.
 * API paths (`/api/v1/...`) are not app routes, so the cast is necessary.
 */
export function apiPath(path: string): string {
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	return resolve(path as any);
}
