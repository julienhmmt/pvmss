import type { APIRequestContext } from '@playwright/test';

/**
 * The CSRF header for a mutating API call, read from the context's pvmss_csrf
 * cookie. Playwright's APIRequestContext shares the browser context's cookies
 * but does not derive the double-submit header the server requires, so a spec
 * that drives /api/v1 directly must add it itself.
 *
 * Returns an empty object when no session has been established (the server
 * skips the CSRF check for cookie-less requests).
 */
export async function csrfHeaders(request: APIRequestContext): Promise<Record<string, string>> {
	const { cookies } = await request.storageState();
	const csrf = cookies.find((cookie) => cookie.name === 'pvmss_csrf');

	return csrf === undefined ? {} : { 'X-CSRF-Token': csrf.value };
}
