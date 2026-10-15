import type { HandleClientError } from '@sveltejs/kit';
import { installClientErrorReporting, reportClientError } from '$lib/shared/api/client-errors';

installClientErrorReporting();

/** SvelteKit funnel for framework errors (load, actions, rendering): forward
 * to the server log, then let the default console reporting happen. */
export const handleError: HandleClientError = ({ error }) => {
	reportClientError(error);
};
