/** One SSH public key stored in the user's profile (GET/POST
 *  /api/v1/profile/ssh-keys). The fingerprint is computed server-side and
 *  never stored; the public key line keeps its original comment. */
export interface ProfileSshKey {
	id: string;
	label: string;
	publicKey: string;
	fingerprint: string;
	createdAt: string;
}

/** A key queued to be saved to the profile after a VM create/update
 *  succeeds (the "Save to my profile" offer). */
export interface ProfileKeyToSave {
	label: string;
	publicKey: string;
}

/** The maximum number of keys a profile can hold (server-enforced). */
export const PROFILE_SSH_KEY_MAX = 10;
