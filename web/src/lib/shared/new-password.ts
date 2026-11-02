import { m } from '#lib/paraglide/messages.js';

const PASSWORD_MIN_BYTES = 8;
const PASSWORD_MAX_BYTES = 72;

/** Validates a new password pair against the server's 8-72 byte policy.
 *  Length is measured in UTF-8 bytes, like the Go len() check, so multi-byte
 *  characters behave identically on both ends. Returns the localized error
 *  to display, or null when the pair is valid. */
export function newPasswordError(password: string, confirmation: string): string | null {
	const bytes = new TextEncoder().encode(password).length;
	if (bytes < PASSWORD_MIN_BYTES) return m['admin.accounts.errorPasswordTooShort']();
	if (bytes > PASSWORD_MAX_BYTES) return m['admin.accounts.errorPasswordTooLong']();
	if (password !== confirmation) return m['admin.accounts.errorPasswordMismatch']();
	return null;
}
