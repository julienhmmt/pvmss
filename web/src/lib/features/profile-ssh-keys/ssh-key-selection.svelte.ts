import { m } from '$lib/paraglide/messages.js';
import { keyComment, keyIdentity } from './ssh-key-identity';
import { PROFILE_SSH_KEY_MAX, type ProfileKeyToSave, type ProfileSshKey } from './types';

/** Identity-based comparison: a profile key whose blob is already among the
 *  existing keys is the same key (comments do not participate). */
function identitySet(keys: string[]): Set<string> {
	return new Set(keys.map((key) => keyIdentity(key)).filter((id) => id !== ''));
}

/** Server-side label cap (1..64 characters). The prefill truncates the
 *  base so base + suffix stays under it. */
const SAVE_LABEL_MAX = 64;

/** Truncates to `max` code points - iterating a string yields code points,
 *  so an emoji at the boundary is never split into a lone surrogate. */
function truncateCodePoints(value: string, max: number): string {
	return [...value].slice(0, max).join('');
}

/** The prefilled save-offer label: `base` (key comment or default label),
 *  suffixed " 2", " 3"... until it is free among the profile labels, with
 *  the base truncated so the result never exceeds SAVE_LABEL_MAX. */
function uniqueSaveLabel(base: string, taken: Set<string>): string {
	for (let suffix = 0; ; suffix++) {
		const tail = suffix === 0 ? '' : ` ${suffix + 1}`;
		const label = truncateCodePoints(base, SAVE_LABEL_MAX - tail.length) + tail;
		if (!taken.has(label)) return label;
	}
}

export interface SshKeySelectionOptions {
	/** VM cloud-init tab mode: the picker only adds keys to a machine that
	 *  already has some - nothing is selected by default and `existing`
	 *  holds the keys currently configured on the VM. */
	addOnly?: boolean;
	/** Key lines already on the machine (VM tab: the live textarea content). */
	existing?: string[];
}

/**
 * Shared SSH-key picker state for the VM creation wizards and the VM
 * cloud-init tab. Holds the profile keys, which ones are picked, the
 * one-off pasted text, and the "save to my profile" offer bookkeeping.
 * Created per form - never a module singleton.
 */
export class SshKeySelection {
	readonly addOnly: boolean;
	profileKeys = $state.raw<ProfileSshKey[]>([]);
	selectedIds = $state.raw<string[]>([]);
	saveToProfile = $state(true);
	/** True once the user typed in the save-label input - a manually edited
	 *  label survives offer-key changes (a fresh offer does not stomp it). */
	saveLabelEdited = $state(false);

	#pasted = $state('');
	#existing = $state.raw<string[]>([]);
	#saveLabel = $state('');
	/** The offer the prefilled saveLabel currently belongs to. */
	#lastOfferKey: string | null = null;

	constructor(options: SshKeySelectionOptions = {}) {
		this.addOnly = options.addOnly ?? false;
		this.#existing = options.existing ?? [];
	}

	/** One-off keys pasted by the user, as raw textarea text. */
	get pasted(): string {
		return this.#pasted;
	}

	set pasted(value: string) {
		this.#pasted = value;
		this.#syncSaveLabel();
	}

	/** Keys already on the machine; deduped into finalKeys first. The VM
	 *  cloud-init form keeps this live from its editable textarea through a
	 *  reactive effect - the equality check makes the write a no-op when the
	 *  lines did not change, which is what lets that effect exist (the
	 *  setter's own reads of #existing would otherwise re-trigger it). */
	get existing(): string[] {
		return this.#existing;
	}

	set existing(keys: string[]) {
		if (keys.length === this.#existing.length && keys.every((key, index) => key === this.#existing[index])) return;
		this.#existing = keys;
		this.#syncSaveLabel();
	}

	/** The save-offer label. Writing through the setter marks the label as
	 *  user-edited; the prefill path writes the backing field directly. */
	get saveLabel(): string {
		return this.#saveLabel;
	}

	set saveLabel(value: string) {
		this.#saveLabel = value;
		this.saveLabelEdited = true;
	}

	/** Stores the loaded profile keys and applies the default selection:
	 *  every key when the profile has 3 or fewer, none otherwise (D8);
	 *  never anything in add-only mode. Keys already on the machine are
	 *  not selectable, so they are never auto-selected either. */
	setProfileKeys(keys: ProfileSshKey[]): void {
		this.profileKeys = keys;
		const onVm = identitySet(this.#existing);
		const selectable = keys.filter((key) => !onVm.has(keyIdentity(key.publicKey)));
		this.selectedIds = !this.addOnly && keys.length <= 3 ? selectable.map((key) => key.id) : [];
		// Forced: the profile may load after the user pasted, so the offer key
		// is unchanged while the label set it must be unique against just did.
		this.#syncSaveLabel(true);
	}

	/** Whether a profile key is already on the machine (shown as present,
	 *  not offered again). */
	isOnVm(key: ProfileSshKey): boolean {
		return identitySet(this.#existing).has(keyIdentity(key.publicKey));
	}

	isSelected(id: string): boolean {
		return this.selectedIds.includes(id);
	}

	toggle(id: string, checked: boolean): void {
		if (checked) {
			if (!this.selectedIds.includes(id)) this.selectedIds = [...this.selectedIds, id];
			return;
		}
		this.selectedIds = this.selectedIds.filter((existing) => existing !== id);
	}

	/** The pasted key lines: newline-split, trimmed, blanks dropped. */
	pastedKeys(): string[] {
		return this.#pasted
			.split('\n')
			.map((key) => key.trim())
			.filter((key) => key !== '');
	}

	/** The final key list sent with the request: existing first, then the
	 *  picked profile keys (in profile order), then the pasted ones -
	 *  deduped by key identity, first occurrence wins. */
	finalKeys(): string[] {
		const out: string[] = [];
		const seen = new Set<string>();
		const push = (key: string): void => {
			const identity = keyIdentity(key);
			if (identity === '' || seen.has(identity)) return;
			seen.add(identity);
			out.push(key);
		};
		for (const key of this.#existing) push(key);
		const picked = new Set(this.selectedIds);
		for (const key of this.profileKeys) {
			if (picked.has(key.id)) push(key.publicKey);
		}
		for (const key of this.pastedKeys()) push(key);
		return out;
	}

	/** Pasted keys known neither to the profile nor to the machine. */
	unknownPastedKeys(): string[] {
		const known = new Set<string>([
			...this.profileKeys.map((key) => keyIdentity(key.publicKey)),
			...identitySet(this.#existing)
		]);
		return this.pastedKeys().filter((key) => !known.has(keyIdentity(key)));
	}

	/** The save offer: the single unknown pasted key when there is exactly
	 *  one and the profile is not full (D9). Several unknown keys or a full
	 *  profile yield no offer. */
	saveOffer(): string | null {
		if (this.profileKeys.length >= PROFILE_SSH_KEY_MAX) return null;
		const unknowns = this.unknownPastedKeys();
		return unknowns.length === 1 ? (unknowns[0] ?? null) : null;
	}

	/** The payload to save to the profile after success, or null when the
	 *  offer is off or has no usable label. */
	keyToSave(): ProfileKeyToSave | null {
		const offer = this.saveOffer();
		if (offer === null || !this.saveToProfile) return null;
		const label = this.saveLabel.trim();
		if (label === '') return null;
		return { label, publicKey: offer };
	}

	/** Clears the picks after a successful save (VM tab): pasted text,
	 *  selected keys, and the whole save-offer state. The loaded profile
	 *  keys and `existing` stay - they describe the world, not the form. */
	reset(): void {
		this.#pasted = '';
		this.selectedIds = [];
		this.saveToProfile = true;
		this.saveLabelEdited = false;
		this.#saveLabel = '';
		this.#lastOfferKey = null;
	}

	/** Recomputes the save offer after an input change; when the offered
	 *  key changed - or `force` is set because the profile label set did -
	 *  reprefills the label unless the user took it over. */
	#syncSaveLabel(force = false): void {
		const offer = this.saveOffer();
		if (!force && offer === this.#lastOfferKey) return;
		this.#lastOfferKey = offer;
		if (this.saveLabelEdited) return;
		if (offer === null) {
			this.#saveLabel = '';
			return;
		}
		const base = keyComment(offer) || m['profileSshKeys.defaultLabel']();
		this.#saveLabel = uniqueSaveLabel(base, new Set(this.profileKeys.map((key) => key.label)));
	}
}
