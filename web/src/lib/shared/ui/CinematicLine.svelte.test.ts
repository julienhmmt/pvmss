import { afterEach, describe, it, expect } from 'vitest';
import { mount } from 'svelte';
import CinematicLine from './CinematicLine.svelte';

describe('CinematicLine', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('runs a calm flow by default', () => {
		mount(CinematicLine, { target: document.body });
		expect(document.querySelector('.pv-cinematic-flow')).not.toBeNull();
		expect(document.querySelector('.pv-cinematic-busy')).toBeNull();
	});

	it('runs fast when busy', () => {
		mount(CinematicLine, { target: document.body, props: { mode: 'busy' } });
		expect(document.querySelector('.pv-cinematic-busy')).not.toBeNull();
	});

	it('is a flat line when still', () => {
		mount(CinematicLine, { target: document.body, props: { mode: 'still' } });
		expect(document.querySelector('.pv-cinematic-flow')).toBeNull();
		expect(document.querySelector('[data-mode="still"]')).not.toBeNull();
	});
});
