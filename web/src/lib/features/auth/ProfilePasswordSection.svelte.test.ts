import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, tick } from 'svelte';
import { setLocale } from '#lib/paraglide/runtime.js';
import ProfilePasswordSection from './ProfilePasswordSection.svelte';
import { SessionStore } from './session.svelte';
import { ToastRegion } from '#lib/shared/ui/toast.svelte.js';

let sessionStore: SessionStore;
let toast: ToastRegion;

vi.mock('./session.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('./session.svelte')>();
	return { ...original, getSessionContext: () => sessionStore };
});

vi.mock('#lib/shared/ui/toast.svelte.js', async (importOriginal) => {
	const original = await importOriginal<typeof import('#lib/shared/ui/toast.svelte.js')>();
	return { ...original, getToastContext: () => toast };
});

async function settle(): Promise<void> {
	for (let i = 0; i < 4; i += 1) {
		await Promise.resolve();
		flushSync();
	}
}

describe('ProfilePasswordSection (Ticket 11)', () => {
	beforeAll(() => setLocale('en', { reload: false }));

	afterEach(() => {
		document.body.innerHTML = '';
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	it('validates minimum length and confirmation match before submitting to POST /api/v1/auth/password', async () => {
		const fetchMock = vi.fn().mockImplementation((input: string) => {
			if (input === '/api/v1/auth/csrf') {
				return Promise.resolve(new Response(JSON.stringify({ token: 'csrf-1' }), { status: 200 }));
			}
			return Promise.resolve(new Response(null, { status: 204 }));
		});
		vi.stubGlobal('fetch', fetchMock);

		sessionStore = new SessionStore();
		sessionStore.principal = {
			username: 'alice@pve',
			displayName: 'alice',
			pool: 'pvmss-alice',
			isAdmin: false,
			cluster: 'default',
			clusterDisplayName: 'Default',
			provider: 'pve',
			isOwner: false
		};
		toast = new ToastRegion();

		mount(ProfilePasswordSection, { target: document.body });

		expect(document.querySelectorAll('[role="alert"]')).toHaveLength(0);

		const submitBtn = document.querySelector<HTMLButtonElement>('[data-testid="profile-password-submit"]');
		expect(submitBtn).not.toBeNull();
		submitBtn!.click();
		flushSync();
		await tick();

		expect(fetchMock).not.toHaveBeenCalled();
		expect(document.body.textContent).toContain('Current password is required.');
		expect(document.body.textContent).toContain('New password must be at least 8 characters.');

		const currentInput = document.querySelector<HTMLInputElement>('[data-testid="profile-password-current"]');
		const newInput = document.querySelector<HTMLInputElement>('[data-testid="profile-password-new"]');
		const confirmInput = document.querySelector<HTMLInputElement>('[data-testid="profile-password-confirm"]');

		currentInput!.value = 'pvmss-alice';
		currentInput!.dispatchEvent(new Event('input', { bubbles: true }));
		newInput!.value = 'new-password-123';
		newInput!.dispatchEvent(new Event('input', { bubbles: true }));
		confirmInput!.value = 'mismatch-123';
		confirmInput!.dispatchEvent(new Event('input', { bubbles: true }));
		flushSync();

		expect(document.body.textContent).toContain('New passwords do not match.');

		confirmInput!.value = 'new-password-123';
		confirmInput!.dispatchEvent(new Event('input', { bubbles: true }));
		flushSync();

		submitBtn!.click();
		await settle();

		expect(fetchMock).toHaveBeenCalled();
		expect(currentInput!.value).toBe('');
		expect(newInput!.value).toBe('');
		expect(confirmInput!.value).toBe('');
		expect(toast.items.some((item) => item.message.includes('Password updated'))).toBe(true);
	});

	it('renders the bootstrap note when signed in as the bootstrap administrator', () => {
		sessionStore = new SessionStore();
		sessionStore.principal = {
			username: 'admin',
			displayName: 'admin',
			pool: '',
			isAdmin: true,
			cluster: '',
			clusterDisplayName: '',
			provider: 'bootstrap',
			isOwner: true
		};
		toast = new ToastRegion();

		mount(ProfilePasswordSection, { target: document.body });

		expect(document.querySelector('[data-testid="profile-local-admin-note"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="profile-password-section"]')).toBeNull();
	});

	it('lets a DB local account change its own password', () => {
		sessionStore = new SessionStore();
		sessionStore.principal = {
			username: 'albert',
			displayName: 'albert',
			pool: '',
			isAdmin: true,
			cluster: '',
			clusterDisplayName: '',
			provider: 'local',
			isOwner: false
		};
		toast = new ToastRegion();

		mount(ProfilePasswordSection, { target: document.body });

		expect(document.querySelector('[data-testid="profile-password-section"]')).not.toBeNull();
		expect(document.querySelector('[data-testid="profile-local-admin-note"]')).toBeNull();
		expect(document.body.textContent).toContain('Update the password PVMSS stores for this local account.');
	});
});
