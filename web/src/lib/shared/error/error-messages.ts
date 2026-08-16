import { m } from '$lib/paraglide/messages.js';

/** Localized title and description for an HTTP error status. */
export interface ErrorMessage {
	title: () => string;
	description: () => string;
}

/** Returns the localized message for a given HTTP status code. */
export function getErrorMessage(status: number): ErrorMessage {
	switch (status) {
		case 404:
			return {
				title: () => m['error.notFoundTitle'](),
				description: () => m['error.notFoundDescription']()
			};
		case 403:
			return {
				title: () => m['error.forbiddenTitle'](),
				description: () => m['error.forbiddenDescription']()
			};
		case 500:
		case 502:
		case 503:
			return {
				title: () => m['error.serverErrorTitle'](),
				description: () => m['error.serverErrorDescription']()
			};
		default:
			return {
				title: () => m['error.genericTitle']({ status: String(status) }),
				description: () => m['error.genericDescription']()
			};
	}
}
