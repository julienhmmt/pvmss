import { afterEach, describe, expect, it, vi } from 'vitest';
import { handleAccepted, type PostSubmitDeps } from './post-submit';
import { TaskTrayStore } from '$lib/features/tasks/tasks.svelte';
import { TaskOutcomeLedger } from '$lib/features/tasks/task-outcome-ledger.svelte';
import { ToastRegion } from '$lib/shared/ui/toast.svelte';
import type { VmCreateAccepted } from './create.svelte';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function accepted(): VmCreateAccepted {
	return { cluster: 'default', vmid: 101, name: 'web-01', node: 'pve', upid: 'UPID:pve:1' };
}

function deps(): PostSubmitDeps & { toast: ToastRegion } {
	return { tray: new TaskTrayStore(), toast: new ToastRegion(), outcomeLedger: new TaskOutcomeLedger() };
}

function profilePosts(fetchMock: ReturnType<typeof vi.fn>): RequestInit[] {
	return fetchMock.mock.calls
		.filter((call) => String(call[0]).includes('/profile/ssh-keys'))
		.map((call) => call[1] as RequestInit);
}

afterEach(() => vi.unstubAllGlobals());

describe('handleAccepted profile-key save', () => {
	it('posts the key to save after the create was accepted', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, {}));
		vi.stubGlobal('fetch', fetchMock);
		const d = deps();

		await handleAccepted(accepted(), { ...d, keyToSave: { label: 'laptop', publicKey: 'ssh-ed25519 AAAA x' } });

		const posts = profilePosts(fetchMock);
		expect(posts).toHaveLength(1);
		expect(posts[0]?.method).toBe('POST');
		expect(JSON.parse(posts[0]?.body as string)).toEqual({ label: 'laptop', publicKey: 'ssh-ed25519 AAAA x' });
	});

	it('does not call the profile API when there is no key to save', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, {}));
		vi.stubGlobal('fetch', fetchMock);

		await handleAccepted(accepted(), deps());

		expect(profilePosts(fetchMock)).toHaveLength(0);
	});

	it('warns by toast but resolves when the profile save fails', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(409, { code: 'duplicate_key', message: 'dup' }));
		vi.stubGlobal('fetch', fetchMock);
		const d = deps();
		const errorSpy = vi.spyOn(d.toast, 'error');

		await expect(
			handleAccepted(accepted(), { ...d, keyToSave: { label: 'laptop', publicKey: 'ssh-ed25519 AAAA x' } })
		).resolves.toBeUndefined();
		// The save is fire-and-forget: let it settle before asserting.
		await vi.waitFor(() => expect(errorSpy).toHaveBeenCalledTimes(1));
		expect(errorSpy.mock.calls[0]?.[0]).not.toContain('dup');
	});

	it.each(['running', 'stopped'] as const)('passes the expected %s status to task-completion listeners', async (expectedStatus) => {
		const d: PostSubmitDeps = deps();
		await handleAccepted({ ...accepted(), expectedStatus }, d);
		expect(d.tray.tasks[0]?.expectedStatus).toBe(expectedStatus);
		d.tray.destroy();
	});

	it('still tracks the task and shows the queued toast alongside the save', async () => {
		const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, {}));
		vi.stubGlobal('fetch', fetchMock);
		const d = deps();
		const trackSpy = vi.spyOn(d.tray, 'track');
		const infoSpy = vi.spyOn(d.toast, 'info');

		await handleAccepted(accepted(), { ...d, keyToSave: { label: 'laptop', publicKey: 'ssh-ed25519 AAAA x' } });

		expect(trackSpy).toHaveBeenCalledTimes(1);
		expect(infoSpy).toHaveBeenCalled();
	});
});
