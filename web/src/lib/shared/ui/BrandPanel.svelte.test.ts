import { afterEach, describe, it, expect } from 'vitest';
import { mount } from 'svelte';
import BrandPanel from './BrandPanel.svelte';

describe('BrandPanel', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders login marketing content by default', () => {
		mount(BrandPanel, { target: document.body });

		expect(document.body.textContent).toContain('Proxmox VM Self-Service (PVMSS)');
	});

	it('renders warning content in warning mode', () => {
		mount(BrandPanel, { target: document.body, props: { mode: 'warning' } });

		expect(document.body.textContent).toContain('Avertissement');
	});

	it('renders the neutral tagline without the login features in error mode', () => {
		mount(BrandPanel, { target: document.body, props: { mode: 'error' } });

		expect(document.body.textContent).toContain('Proxmox VM Self-Service (PVMSS)');
		expect(document.body.querySelector('ul')).toBeNull();
	});
});
