import { describe, it, expect, vi } from 'vitest';
import { mount } from 'svelte';
import AuthRequired from './AuthRequired.svelte';

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

describe('AuthRequired', () => {
	it('renders the authentication warning title and sign-in button', () => {
		mount(AuthRequired, { target: document.body });

		expect(document.body.textContent).toContain('Authentification requise');
		expect(document.body.textContent).toContain('Se connecter');
	});
});
