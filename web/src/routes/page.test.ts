import { describe, it, expect } from 'vitest';
import { mount } from 'svelte';
import TestWrapper from '../test/TestWrapper.svelte';

describe('PVMSS web shell smoke test', () => {
	it('renders the placeholder heading in the layout', () => {
		mount(TestWrapper, { target: document.body });

		expect(document.body.textContent ?? '').toContain('Proxmox VM Self-Service (PVMSS)');
	});
});
