import { afterEach, describe, expect, it, vi, type Mock } from 'vitest';
import { mount } from 'svelte';
import CloudInitTab from './CloudInitTab.svelte';
import { VmDetailStore } from './detail.svelte';
import { SshKeySelection } from '$lib/features/profile-ssh-keys/ssh-key-selection.svelte';
import type { ProfileSshKey } from '$lib/features/profile-ssh-keys/types';
import { ToastRegion } from '$lib/shared/ui/toast.svelte';

let vmStoreInstance: VmDetailStore;
const toast = new ToastRegion();

vi.mock('./detail.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('./detail.svelte')>();
	return { ...original, getVmDetailContext: () => vmStoreInstance };
});

vi.mock('$lib/shared/ui/toast.svelte', async (importOriginal) => {
	const original = await importOriginal<typeof import('$lib/shared/ui/toast.svelte')>();
	return { ...original, getToastContext: () => toast };
});

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

const config = { user: 'debian', sshKeys: ['ssh-ed25519 OLD on-vm'], ipMode: 'dhcp' };
const profileKeys: ProfileSshKey[] = [];

interface Router {
	saveFails: boolean;
}

type FetchFn = (input: string | URL | Request, init?: RequestInit) => Promise<Response>;

function urlOf(input: string | URL | Request): string {
	return typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
}

function routedFetch(router: Router): Mock<FetchFn> {
	return vi.fn<FetchFn>((input, init) => {
		const url = urlOf(input);
		if (url.includes('/profile/ssh-keys')) {
			if (init?.method === 'POST') return Promise.resolve(jsonResponse(201, {}));
			return Promise.resolve(jsonResponse(200, { keys: profileKeys }));
		}
		if (url.endsWith('/cloudinit')) {
			if (init?.method === 'PUT') {
				return router.saveFails
					? Promise.resolve(jsonResponse(500, { code: 'internal_error', message: 'boom' }))
					: Promise.resolve(jsonResponse(200, { status: 'updated', rebooted: false }));
			}
			return Promise.resolve(jsonResponse(200, config));
		}
		return Promise.resolve(jsonResponse(200, {}));
	});
}

async function openSaveDialog(): Promise<void> {
	await vi.waitFor(() => {
		const button = document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-save"]');
		expect(button).not.toBeNull();
		expect(button?.disabled).toBe(false);
	});
	// happy-dom does not synthesize form submission from a submit-button
	// click - dispatch the submit event the browser would produce.
	document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
	await vi.waitFor(() => expect(document.querySelector('[data-testid="cloudinit-save-confirm"]')).not.toBeNull());
}

function profilePosts(fetchMock: Mock<FetchFn>): RequestInit[] {
	return fetchMock.mock.calls
		.filter((call) => urlOf(call[0]).includes('/profile/ssh-keys') && call[1]?.method === 'POST')
		.map((call) => call[1] as RequestInit);
}

afterEach(() => {
	document.body.innerHTML = '';
	vi.unstubAllGlobals();
});

describe('CloudInitTab profile-key save offer', () => {
	it('posts the offered key to the profile after the update succeeds and resets the picker', async () => {
		const fetchMock = routedFetch({ saveFails: false });
		vi.stubGlobal('fetch', fetchMock);
		vmStoreInstance = new VmDetailStore('default', 101);
		const sshSelection = new SshKeySelection({ addOnly: true });
		sshSelection.pasted = 'ssh-ed25519 NEW work-laptop';

		mount(CloudInitTab, { target: document.body, props: { sshSelection } });
		await openSaveDialog();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-save-confirm"]')!.click();
		await vi.waitFor(() => expect(profilePosts(fetchMock)).toHaveLength(1));

		const body = JSON.parse(profilePosts(fetchMock)[0]?.body as string) as { label: string; publicKey: string };
		expect(body).toEqual({ label: 'work-laptop', publicKey: 'ssh-ed25519 NEW work-laptop' });
		expect(sshSelection.pasted).toBe('');
		expect(sshSelection.selectedIds).toEqual([]);
		// The cloud-init PUT carried the pasted key into the VM's key list.
		const put = fetchMock.mock.calls.find((call) => call[1]?.method === 'PUT');
		const putBody = JSON.parse(put?.[1]?.body as string) as { sshKeys?: string[] };
		expect(putBody.sshKeys).toContain('ssh-ed25519 NEW work-laptop');
	});

	it('does not call the profile API when the update fails', async () => {
		const fetchMock = routedFetch({ saveFails: true });
		vi.stubGlobal('fetch', fetchMock);
		vmStoreInstance = new VmDetailStore('default', 101);
		const sshSelection = new SshKeySelection({ addOnly: true });
		sshSelection.pasted = 'ssh-ed25519 NEW work-laptop';

		mount(CloudInitTab, { target: document.body, props: { sshSelection } });
		await openSaveDialog();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-save-confirm"]')!.click();
		await vi.waitFor(() => {
			const put = fetchMock.mock.calls.find((call) => call[1]?.method === 'PUT');
			expect(put).not.toBeUndefined();
		});
		// Give the rejected save a chance to wrongly fire the profile POST.
		await new Promise((resolve) => setTimeout(resolve, 20));

		expect(profilePosts(fetchMock)).toHaveLength(0);
	});

	it('does not call the profile API when the pasted key is already in the profile', async () => {
		const fetchMock = routedFetch({ saveFails: false });
		vi.stubGlobal('fetch', fetchMock);
		vmStoreInstance = new VmDetailStore('default', 101);
		const sshSelection = new SshKeySelection({ addOnly: true });
		// Same blob as an existing VM key: it is "known", so no save offer.
		sshSelection.pasted = 'ssh-ed25519 OLD other-comment';

		mount(CloudInitTab, { target: document.body, props: { sshSelection } });
		await openSaveDialog();

		document.querySelector<HTMLButtonElement>('[data-testid="cloudinit-save-confirm"]')!.click();
		await vi.waitFor(() => {
			const put = fetchMock.mock.calls.find((call) => call[1]?.method === 'PUT');
			expect(put).not.toBeUndefined();
		});
		await new Promise((resolve) => setTimeout(resolve, 20));

		expect(profilePosts(fetchMock)).toHaveLength(0);
	});
});
