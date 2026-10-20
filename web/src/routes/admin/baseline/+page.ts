import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Cloud-init is paused for now: not reliable or viable yet, too complex to set up.
// Delete this file to bring the page back (with cloudInitPaused in server/internal/httpapi/router.go).
export function load(): never {
	redirect(307, resolve('/admin'));
}
