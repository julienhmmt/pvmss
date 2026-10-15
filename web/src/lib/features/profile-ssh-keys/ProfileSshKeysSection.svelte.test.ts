import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount, flushSync } from 'svelte';
import ProfileSshKeysSection from './ProfileSshKeysSection.svelte';
import { profileKeyErrorMessage } from './ssh-key-identity';
import { PROFILE_SSH_KEY_MAX, type ProfileSshKey } from './types';
import { m } from '$lib/paraglide/messages.js';

function jsonResponse(status: number, body: unknown): Response {
	return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function key(id: string, label: string): ProfileSshKey {
	return {
		id,
		label,
		publicKey: `ssh-ed25519 BLOB${id} comment`,
		fingerprint: `SHA256:${id}`,
		createdAt: '2026-09-29T10:00:00Z'
	};
}

function mountSection(): void {
	mount(ProfileSshKeysSection, { target: document.body });
}

async function waitFor(assertion: () => void): Promise<void> {
	await vi.waitFor(assertion, { timeout: 2000 });
}

function typeLabel(value: string): void {
	const input = document.querySelector<HTMLInputElement>('[data-testid="ssh-key-add-label"]')!;
	input.value = value;
	input.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
}

function typeKey(value: string): void {
	const textarea = document.querySelector<HTMLTextAreaElement>('[data-testid="ssh-key-add-key"]')!;
	textarea.value = value;
	textarea.dispatchEvent(new Event('input', { bubbles: true }));
	flushSync();
}

function clickAdd(): void {
	document.querySelector<HTMLButtonElement>('[data-testid="ssh-key-add"]')!.click();
	flushSync();
}

afterEach(() => {
	document.body.innerHTML = '';
	vi.unstubAllGlobals();
});

describe('ProfileSshKeysSection', () => {
	it('lists the saved keys with label, fingerprint and creation date', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, { keys: [key('k1', 'laptop'), key('k2', 'desktop')] }))
		);
		mountSection();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-key-row-k1"]')).not.toBeNull());

		const row = document.querySelector('[data-testid="ssh-key-row-k1"]')!;
		expect(row.textContent).toContain('laptop');
		expect(row.textContent).toContain('SHA256:k1');
		expect(row.textContent).toContain('2026');
		expect(document.querySelector('[data-testid="ssh-key-row-k2"]')?.textContent).toContain('desktop');
	});

	it('shows the empty state when the profile has no key', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys: [] })));
		mountSection();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-keys-empty"]')).not.toBeNull());
		expect(document.querySelector('[data-testid="ssh-keys-empty"]')?.textContent).toContain(m['profileSshKeys.empty']());
	});

	it('shows the load error when the list cannot be fetched', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { code: 'internal_error', message: 'x' })));
		mountSection();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-keys-load-error"]')).not.toBeNull());
		expect(document.querySelector('[data-testid="ssh-keys-load-error"]')?.textContent).toContain(
			m['profileSshKeys.loadError']()
		);
	});

	it('adds a key and shows it in the list', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { keys: [] }))
			.mockResolvedValueOnce(jsonResponse(201, key('k9', 'new key')));
		vi.stubGlobal('fetch', fetchMock);
		mountSection();
		await waitFor(() => expect(document.querySelector('[data-testid="ssh-keys-empty"]')).not.toBeNull());

		typeLabel('new key');
		typeKey('ssh-ed25519 BLOBk9 comment');
		clickAdd();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-key-row-k9"]')).not.toBeNull());
		const init = fetchMock.mock.calls[1]?.[1] as RequestInit | undefined;
		expect(init?.method).toBe('POST');
		expect(JSON.parse(init?.body as string)).toEqual({ label: 'new key', publicKey: 'ssh-ed25519 BLOBk9 comment' });
	});

	const errorCases: Array<[string, number]> = [
		['invalid_label', 400],
		['invalid_request', 400],
		['ssh_key_empty', 400],
		['ssh_key_multiline', 400],
		['ssh_key_type', 400],
		['ssh_key_format', 400],
		['ssh_key_private', 400],
		['ssh_key_too_long', 400],
		['duplicate_label', 409],
		['duplicate_key', 409],
		['limit_reached', 409]
	];

	it.each(errorCases)('shows the translated message for %s', async (code, status) => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { keys: [] }))
			.mockResolvedValueOnce(jsonResponse(status, { code, message: 'raw server text' }));
		vi.stubGlobal('fetch', fetchMock);
		mountSection();
		await waitFor(() => expect(document.querySelector('[data-testid="ssh-keys-empty"]')).not.toBeNull());

		typeLabel('x');
		typeKey('ssh-ed25519 BLOB c');
		clickAdd();

		await waitFor(() => expect(document.body.textContent).toContain(profileKeyErrorMessage(code)));
		expect(document.body.textContent).not.toContain('raw server text');
	});

	it('deletes a key after confirmation', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { keys: [key('k1', 'laptop')] }))
			.mockResolvedValueOnce(new Response(null, { status: 204 }));
		vi.stubGlobal('fetch', fetchMock);
		mountSection();
		await waitFor(() => expect(document.querySelector('[data-testid="ssh-key-row-k1"]')).not.toBeNull());

		document.querySelector<HTMLButtonElement>('[data-testid="ssh-key-delete-k1"]')!.click();
		flushSync();
		document.querySelector<HTMLButtonElement>('[data-testid="ssh-key-delete-confirm"]')!.click();
		flushSync();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-key-row-k1"]')).toBeNull());
		const req = fetchMock.mock.calls[1] as [string, RequestInit | undefined] | undefined;
		expect(String(req?.[0])).toContain('/api/v1/profile/ssh-keys/k1');
		expect(req?.[1]?.method).toBe('DELETE');
	});

	it('keeps the key and reports the error when deletion fails', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(jsonResponse(200, { keys: [key('k1', 'laptop')] }))
			.mockResolvedValueOnce(jsonResponse(404, { code: 'not_found', message: 'gone' }));
		vi.stubGlobal('fetch', fetchMock);
		mountSection();
		await waitFor(() => expect(document.querySelector('[data-testid="ssh-key-row-k1"]')).not.toBeNull());

		document.querySelector<HTMLButtonElement>('[data-testid="ssh-key-delete-k1"]')!.click();
		flushSync();
		document.querySelector<HTMLButtonElement>('[data-testid="ssh-key-delete-confirm"]')!.click();
		flushSync();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-keys-delete-error"]')).not.toBeNull());
		expect(document.querySelector('[data-testid="ssh-key-row-k1"]')).not.toBeNull();
	});

	it('disables the add form at the key limit', async () => {
		const keys = Array.from({ length: PROFILE_SSH_KEY_MAX }, (_, i) => key(`k${i}`, `key ${i}`));
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { keys })));
		mountSection();

		await waitFor(() => expect(document.querySelector('[data-testid="ssh-keys-limit"]')).not.toBeNull());
		expect(document.querySelector<HTMLInputElement>('[data-testid="ssh-key-add-label"]')?.disabled).toBe(true);
		expect(document.querySelector<HTMLTextAreaElement>('[data-testid="ssh-key-add-key"]')?.disabled).toBe(true);
		expect(document.querySelector<HTMLButtonElement>('[data-testid="ssh-key-add"]')?.disabled).toBe(true);
	});
});
