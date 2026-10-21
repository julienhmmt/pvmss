import { afterEach, describe, it, expect, vi } from 'vitest';
import { mount } from 'svelte';
import ErrorScreen from './ErrorScreen.svelte';
import { m } from '#lib/paraglide/messages.js';

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

describe('ErrorScreen', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders the 404 title and back-to-home button', () => {
		mount(ErrorScreen, { target: document.body, props: { status: 404 } });

		expect(document.body.textContent).toContain('Page introuvable');
		expect(document.body.textContent).toContain('Retour');
	});

	it('renders a generic status title for an unknown code', () => {
		mount(ErrorScreen, { target: document.body, props: { status: 418 } });

		expect(document.body.textContent).toContain('418');
	});

	it('shows one localized 404 message and no generic failure claim', () => {
		mount(ErrorScreen, { target: document.body, props: { status: 404 } });

		expect(document.body.textContent).toContain(m['error.notFoundDescription']());
		expect(document.body.textContent).not.toContain('Not Found');
		expect(document.body.textContent).not.toContain(m['error.genericDescription']());
	});
});
