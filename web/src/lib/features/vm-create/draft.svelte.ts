import { getContext, setContext } from 'svelte';
import { SvelteDate } from 'svelte/reactivity';
import type { CreateMode, VmSource } from './create.svelte';

export const DRAFT_STORAGE_KEY = 'pvmss-vm-create-draft';
export const DRAFT_SCHEMA_VERSION = 4;

const SAVE_DEBOUNCE_MS = 400;

/** One NIC row persisted in the draft (mirrors NICRow from create.svelte). */
export interface DraftNIC {
	bridge: string;
	model: string;
}

/** The form's persistable field values — VMCreateRequest's client-side twin. */
export interface DraftValues {
	mode: CreateMode;
	name: string;
	profileId: string;
	cloudInitTemplateId?: string;
	node: string;
	nodeAdjusted: boolean;
	storage: string;
	storageAdjusted: boolean;
	tagsInput: string;
	sockets?: number;
	cpuCores: number;
	memoryMB: number;
	diskSizeGB: number;
	diskStorage: string;
	nics?: DraftNIC[];
	isoFile: string;
	sourceType?: VmSource;
	templateId?: number;
	templateMinDiskGB?: number;
	startAfterCreate: boolean;
	uefi?: boolean;
	tpm?: boolean;
}

export interface StoredDraft {
	schemaVersion: number;
	values: DraftValues;
	savedAt: string;
}

/**
 * Versioned localStorage draft for the create form (V10, FR-019..FR-021).
 * Comfort-only, browser-local, ephemeral: a version mismatch means "start
 * over", never "attempt to merge" (plan.md research decisions). Writes are
 * debounced; a successful creation clears the draft immediately.
 */
export class DraftStore {
	#restoreToastPending = $state(false);
	#saveTimer: ReturnType<typeof setTimeout> | null = null;

	/** Reads the stored draft. A matching schema version restores the values
	 *  and arms the "draft restored" toast; a mismatch or absence discards
	 *  silently (FR-020). */
	load(): StoredDraft | null {
		const raw = localStorage.getItem(DRAFT_STORAGE_KEY);
		if (raw === null) return null;
		let parsed: StoredDraft;
		try {
			parsed = JSON.parse(raw) as StoredDraft;
		} catch {
			localStorage.removeItem(DRAFT_STORAGE_KEY);
			return null;
		}
		if (parsed.schemaVersion !== DRAFT_SCHEMA_VERSION) {
			localStorage.removeItem(DRAFT_STORAGE_KEY);
			return null;
		}
		this.#restoreToastPending = true;
		return parsed;
	}

	/** True exactly once after a successful version-matched load. */
	consumeRestoreToast(): boolean {
		const pending = this.#restoreToastPending;
		this.#restoreToastPending = false;
		return pending;
	}

	/** Debounced persist on field change (FR-019) — only the latest values
	 *  within the debounce window are written. */
	scheduleSave(values: DraftValues): void {
		if (this.#saveTimer !== null) clearTimeout(this.#saveTimer);
		this.#saveTimer = setTimeout(() => {
			this.#saveTimer = null;
			this.saveNow(values);
		}, SAVE_DEBOUNCE_MS);
	}

	/** Writes the draft now, cancelling any pending debounced write. */
	saveNow(values: DraftValues): void {
		if (this.#saveTimer !== null) clearTimeout(this.#saveTimer);
		this.#saveTimer = null;
		const draft: StoredDraft = {
			schemaVersion: DRAFT_SCHEMA_VERSION,
			values,
			savedAt: new SvelteDate().toISOString()
		};
		localStorage.setItem(DRAFT_STORAGE_KEY, JSON.stringify(draft));
	}

	/** Clears the draft — called the instant a creation succeeds, before any
	 *  navigation (FR-021). */
	clear(): void {
		if (this.#saveTimer !== null) clearTimeout(this.#saveTimer);
		this.#saveTimer = null;
		localStorage.removeItem(DRAFT_STORAGE_KEY);
	}

	/** True if a draft is currently stored (used by the unsaved-changes guard). */
	hasDraft(): boolean {
		return localStorage.getItem(DRAFT_STORAGE_KEY) !== null;
	}
}

const DRAFT_CONTEXT_KEY = Symbol('vm-create-draft');

/** Called once, by the create route. */
export function setDraftContext(): DraftStore {
	const store = new DraftStore();
	setContext(DRAFT_CONTEXT_KEY, store);
	return store;
}

export function getDraftContext(): DraftStore {
	return getContext<DraftStore>(DRAFT_CONTEXT_KEY);
}
