import { post, ApiRequestError } from '$lib/shared/api/client';
import { apiPath } from '$lib/shared/api/paths';
import { m } from '$lib/paraglide/messages.js';
import type { ToastRegion } from '$lib/shared/ui/toast.svelte';
import { profileKeyErrorMessage } from './ssh-key-identity';
import type { ProfileKeyToSave, ProfileSshKey } from './types';

/**
 * Saves a "Save to my profile" offer after a VM create or cloud-init
 * update succeeded (D9). Best-effort by contract: a failure shows a
 * translated warning toast and resolves - it never throws, never blocks
 * the caller's success path, and never un-creates the VM.
 */
export async function saveProfileKeyAfterSuccess(toast: ToastRegion, key: ProfileKeyToSave | null): Promise<void> {
	if (key === null) return;
	try {
		await post<ProfileSshKey>(apiPath('/api/v1/profile/ssh-keys'), { label: key.label, publicKey: key.publicKey });
	} catch (error: unknown) {
		const code = error instanceof ApiRequestError ? error.code : null;
		toast.error(m['profileSshKeys.saveFailed']({ reason: profileKeyErrorMessage(code) }));
	}
}
