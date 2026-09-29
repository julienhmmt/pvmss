import { get, post, del, ApiRequestError } from '$lib/shared/api/client';
import { apiPath } from '$lib/shared/api/paths';
import { m } from '$lib/paraglide/messages.js';
import { PROFILE_SSH_KEY_MAX, type ProfileSshKey } from './types';
import type { SshKeySelection } from './ssh-key-selection.svelte';

const ENDPOINT = '/api/v1/profile/ssh-keys';

interface ProfileSshKeyListResponse {
	keys: ProfileSshKey[];
}

export type AddResult = { ok: true } | { ok: false; code: string | null };

/**
 * Store for the user's profile SSH keys (/profile section). One instance
 * per screen - created by the section component, not a module singleton.
 */
export class ProfileSshKeysStore {
	keys = $state.raw<ProfileSshKey[]>([]);
	loading = $state.raw(false);
	loadError = $state.raw<string | null>(null);
	/** True once the first load settled - the UI shows skeletons until then
	 *  so an empty profile never flashes "no keys" while loading. */
	loaded = $state.raw(false);

	limitReached = $derived(this.keys.length >= PROFILE_SSH_KEY_MAX);

	async load(): Promise<void> {
		this.loading = true;
		this.loadError = null;
		try {
			const response = await get<ProfileSshKeyListResponse>(apiPath(ENDPOINT));
			this.keys = response.keys;
		} catch {
			this.keys = [];
			this.loadError = m['profileSshKeys.loadError']();
		} finally {
			this.loading = false;
			this.loaded = true;
		}
	}

	/** Adds a key; on success the returned row is appended so the list does
	 *  not need a reload. */
	async add(label: string, publicKey: string): Promise<AddResult> {
		try {
			const created = await post<ProfileSshKey>(apiPath(ENDPOINT), { label, publicKey });
			this.keys = [...this.keys, created];
			return { ok: true };
		} catch (error: unknown) {
			return { ok: false, code: error instanceof ApiRequestError ? error.code : null };
		}
	}

	/** Deletes a key by id. Returns false (and keeps the row) on failure. */
	async remove(id: string): Promise<boolean> {
		try {
			await del<undefined>(apiPath(`${ENDPOINT}/${encodeURIComponent(id)}`));
			this.keys = this.keys.filter((key) => key.id !== id);
			return true;
		} catch {
			return false;
		}
	}
}

/** Loads the profile keys into a picker selection (wizards, VM cloud-init
 *  tab). Best-effort by design: any failure behaves as an empty profile so
 *  the picker degrades to the textarea-only experience. */
export async function loadProfileSshKeys(selection: SshKeySelection): Promise<void> {
	try {
		const response = await get<ProfileSshKeyListResponse>(apiPath(ENDPOINT));
		selection.setProfileKeys(response.keys);
	} catch {
		selection.setProfileKeys([]);
	}
}
