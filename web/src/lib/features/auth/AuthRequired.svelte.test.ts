import { describe, it, expect, vi } from 'vitest';
import { mount } from 'svelte';
import { goto } from '$app/navigation';
import AuthRequired from './AuthRequired.svelte';

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));
vi.mock('$app/state', () => ({ page: { url: new URL('http://localhost/vms?cluster=default') } }));

describe('AuthRequired', () => {
	it('renders the authentication warning title and sign-in button', () => {
		mount(AuthRequired, { target: document.body });

		expect(document.body.textContent).toContain('Authentification requise');
		expect(document.body.textContent).toContain('Se connecter');
	});

	it('sends the user to sign in with the current page as next', () => {
		mount(AuthRequired, { target: document.body });
		document.querySelector('button')?.click();

		expect(goto).toHaveBeenCalledWith('/login?next=%2Fvms%3Fcluster%3Ddefault');
	});
});
