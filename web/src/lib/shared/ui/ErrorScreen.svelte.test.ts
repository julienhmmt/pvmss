import { describe, it, expect, vi } from 'vitest';
import { mount } from 'svelte';
import ErrorScreen from './ErrorScreen.svelte';

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

describe('ErrorScreen', () => {
	it('renders the 404 title and back-to-home button', () => {
		mount(ErrorScreen, { target: document.body, props: { status: 404 } });

		expect(document.body.textContent).toContain('Page introuvable');
		expect(document.body.textContent).toContain('Retour');
	});

	it('renders a generic status title for an unknown code', () => {
		mount(ErrorScreen, { target: document.body, props: { status: 418 } });

		expect(document.body.textContent).toContain('418');
	});

	it('uses the provided message over the default description', () => {
		mount(ErrorScreen, { target: document.body, props: { status: 500, message: 'Database is down' } });

		expect(document.body.textContent).toContain('Database is down');
	});
});
