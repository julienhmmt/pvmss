import { beforeEach, describe, expect, it } from 'vitest';
import { mount } from 'svelte';
import DashboardOnboarding from './DashboardOnboarding.svelte';
import { nextOnboardingStep, onboardingHref, type OnboardingStep } from './onboarding';

const steps: OnboardingStep[] = [
	{ key: 'nodes', done: true, required: true, count: 2 },
	{ key: 'storages', done: false, required: true, count: 0, clusterKey: 'east' },
	{ key: 'profiles', done: false, required: true, count: 0, clusterKey: 'east' },
	{ key: 'policy', done: false, required: false, count: 0, clusterKey: 'east' }
];

beforeEach(() => {
	document.body.innerHTML = '';
});

describe('onboarding helpers', () => {
	it('links an undone step to its page filtered on the cluster', () => {
		expect(onboardingHref(steps[1]!)).toBe('/admin/storages?cluster=east');
		expect(onboardingHref(steps[2]!)).toBe('/admin/profiles');
	});

	it('picks the first undone required step as next', () => {
		expect(nextOnboardingStep(steps)?.key).toBe('storages');
		expect(nextOnboardingStep([steps[0]!, steps[3]!])).toBeUndefined();
	});
});

describe('DashboardOnboarding', () => {
	it('lists every step, counts only required ones and links undone steps', () => {
		mount(DashboardOnboarding, { target: document.body, props: { steps, onMarkDone: () => Promise.resolve() } });

		const rows = document.querySelectorAll('[data-testid="onboarding-step"]');
		expect(rows).toHaveLength(4);
		expect(rows.item(0)?.querySelector('a')).toBeNull();
		expect(rows.item(1)?.querySelector('a')?.getAttribute('aria-current')).toBe('step');
		expect(rows.item(2)?.querySelector('a')?.getAttribute('aria-current')).toBeNull();
		expect(document.querySelector('[data-testid="onboarding-progress"]')?.textContent).toMatch(/1\D+3/);
	});

	it('marks one step done through its button', async () => {
		const marked: string[][] = [];
		mount(DashboardOnboarding, {
			target: document.body,
			props: { steps, onMarkDone: (keys: string[]) => { marked.push(keys); return Promise.resolve(); } }
		});

		const buttons = document.querySelectorAll('[data-testid="onboarding-mark-done"]');
		expect(buttons).toHaveLength(3);
		(buttons.item(0) as HTMLButtonElement).click();
		await Promise.resolve();
		expect(marked).toEqual([['storages']]);
	});

	it('marks every undone step through mark all done', async () => {
		const marked: string[][] = [];
		mount(DashboardOnboarding, {
			target: document.body,
			props: { steps, onMarkDone: (keys: string[]) => { marked.push(keys); return Promise.resolve(); } }
		});

		(document.querySelector('[data-testid="onboarding-mark-all"]') as HTMLButtonElement).click();
		await Promise.resolve();
		expect(marked).toEqual([['storages', 'profiles', 'policy']]);
	});
});
