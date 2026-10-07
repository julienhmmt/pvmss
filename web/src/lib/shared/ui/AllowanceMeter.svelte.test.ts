import { describe, it, expect, afterEach } from 'vitest';
import { mount } from 'svelte';
import AllowanceMeter from './AllowanceMeter.svelte';

describe('AllowanceMeter', () => {
	afterEach(() => {
		document.body.innerHTML = '';
	});

	it('renders the correct number of segments', () => {
		mount(AllowanceMeter, {
			target: document.body,
			props: { used: 2, limit: 5, label: '2 of 5 machines used' }
		});
		const segments = document.body.querySelectorAll('[role="meter"] > span');
		expect(segments.length).toBe(5);
	});

	it('fills exactly `used` segments', () => {
		mount(AllowanceMeter, {
			target: document.body,
			props: { used: 3, limit: 5, label: '3 of 5' }
		});
		const filled = document.body.querySelectorAll('[role="meter"] > span.bg-primary');
		const empty = document.body.querySelectorAll('[role="meter"] > span.bg-muted');
		expect(filled.length).toBe(3);
		expect(empty.length).toBe(2);
	});

	it('clamps used above limit to limit', () => {
		mount(AllowanceMeter, {
			target: document.body,
			props: { used: 99, limit: 5, label: 'over' }
		});
		const filled = document.body.querySelectorAll('[role="meter"] > span.bg-primary');
		expect(filled.length).toBe(5);
		expect((document.body.querySelector('[role="meter"]') as HTMLElement).getAttribute('aria-valuenow')).toBe('5');
	});

	it('clamps used below zero to zero', () => {
		mount(AllowanceMeter, {
			target: document.body,
			props: { used: -3, limit: 5, label: 'under' }
		});
		const filled = document.body.querySelectorAll('[role="meter"] > span.bg-primary');
		expect(filled.length).toBe(0);
		expect((document.body.querySelector('[role="meter"]') as HTMLElement).getAttribute('aria-valuenow')).toBe('0');
	});

	it('sets the aria attributes correctly', () => {
		mount(AllowanceMeter, {
			target: document.body,
			props: { used: 2, limit: 5, label: '2 of 5 machines used' }
		});
		const meter = document.body.querySelector('[role="meter"]') as HTMLElement;
		expect(meter.getAttribute('aria-valuenow')).toBe('2');
		expect(meter.getAttribute('aria-valuemin')).toBe('0');
		expect(meter.getAttribute('aria-valuemax')).toBe('5');
		expect(meter.getAttribute('aria-label')).toBe('2 of 5 machines used');
	});

	it('renders zero filled segments when used is zero', () => {
		mount(AllowanceMeter, {
			target: document.body,
			props: { used: 0, limit: 4, label: '0 of 4' }
		});
		const filled = document.body.querySelectorAll('[role="meter"] > span.bg-primary');
		expect(filled.length).toBe(0);
	});
});
