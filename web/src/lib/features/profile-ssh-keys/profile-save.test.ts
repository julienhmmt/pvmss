import { afterEach, describe, expect, it, vi } from 'vitest';
import { saveProfileKeyAfterSuccess } from './profile-save';
import { ToastRegion } from '$lib/shared/ui/toast.svelte';
import { m } from '$lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('saveProfileKeyAfterSuccess', () => {
	it('does nothing when there is no key to save', async () => {
		const fetchMock = vi.fn();
		vi.stubGlobal('fetch', fetchMock);
		const toast = new ToastRegion();

		await saveProfileKeyAfterSuccess(toast, null);

		expect(fetchMock).not.toHaveBeenCalled();
		expect(toast.items).toHaveLength(0);
	});

	it('posts the key to the profile endpoint', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(jsonResponse(201, { id: 'k1', label: 'laptop', publicKey: 'ssh-ed25519 B', fingerprint: 'SHA256:x', createdAt: 't' }));
		vi.stubGlobal('fetch', fetchMock);
		const toast = new ToastRegion();

		await saveProfileKeyAfterSuccess(toast, { label: 'laptop', publicKey: 'ssh-ed25519 B' });

		const req = fetchMock.mock.calls[0] as [string, RequestInit | undefined] | undefined;
		expect(String(req?.[0])).toContain('/api/v1/profile/ssh-keys');
		expect(req?.[1]?.method).toBe('POST');
		expect(JSON.parse(req?.[1]?.body as string)).toEqual({ label: 'laptop', publicKey: 'ssh-ed25519 B' });
		expect(toast.items).toHaveLength(0);
	});

	it('shows a non-blocking translated warning on failure and never throws', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(409, { code: 'duplicate_key', message: 'server text' })));
		const toast = new ToastRegion();

		await expect(
			saveProfileKeyAfterSuccess(toast, { label: 'laptop', publicKey: 'ssh-ed25519 B' })
		).resolves.toBeUndefined();

		expect(toast.items).toHaveLength(1);
		const item = toast.items[0]!;
		expect(item.variant).toBe('error');
		expect(item.message).toContain(m['profileSshKeys.error.duplicateKey']());
		expect(item.message).not.toContain('server text');
	});

	it('survives a network failure with the generic message', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
		const toast = new ToastRegion();

		await expect(
			saveProfileKeyAfterSuccess(toast, { label: 'x', publicKey: 'ssh-ed25519 B' })
		).resolves.toBeUndefined();

		expect(toast.items).toHaveLength(1);
		expect(toast.items[0]?.message).toContain(m['profileSshKeys.error.generic']());
	});
});
