import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';

// Cloud images (.img) are disabled for now: they do not work reliably yet.
// Delete this file to bring the page back (with its routes in router_admin.go).
export function load(): never {
	redirect(307, resolve('/admin/isos'));
}
