import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DraftStore, DRAFT_SCHEMA_VERSION, DRAFT_STORAGE_KEY } from './draft.svelte';
import type { DraftValues } from './draft.svelte';

// T035/T036 (US3): versioned localStorage draft — restore on match, silent
// discard on mismatch, clear on successful creation (FR-019..FR-021).

function sampleValues(): DraftValues {
	return {
		mode: 'detailed',
		name: 'web-09',
		profileId: '',
		node: 'pve-node-02',
		nodeAdjusted: false,
		storage: 'ceph-data',
		storageAdjusted: false,
		tagsInput: 'team-web',
		sockets: 2,
		cpuCores: 2,
		memoryMB: 4096,
		diskSizeGB: 40,
		diskStorage: 'ceph-data',
		nics: [{ bridge: 'vmbr0', model: 'virtio' }],
		isoFile: '',
		startAfterCreate: true
	};
}

function seedDraft(version: number): void {
	localStorage.setItem(
		DRAFT_STORAGE_KEY,
		JSON.stringify({ schemaVersion: version, values: sampleValues(), savedAt: new Date().toISOString() })
	);
}

describe('DraftStore.load', () => {
	beforeEach(() => localStorage.clear());

	it('matching schema version restores the values and announces a toast', () => {
		seedDraft(DRAFT_SCHEMA_VERSION);
		const draft = new DraftStore();

		const restored = draft.load();

		expect(restored).not.toBeNull();
		expect(restored?.values).toEqual(sampleValues());
		expect(restored?.savedAt).toBeTypeOf('string');
		expect(draft.consumeRestoreToast()).toBe(true);
	});

	it('mismatched schema version discards silently — no values, no toast, draft removed', () => {
		seedDraft(DRAFT_SCHEMA_VERSION + 1);
		const draft = new DraftStore();

		const restored = draft.load();

		expect(restored).toBeNull();
		expect(draft.consumeRestoreToast()).toBe(false);
		expect(localStorage.getItem(DRAFT_STORAGE_KEY)).toBeNull();
	});

	it('absent draft is a no-op', () => {
		const draft = new DraftStore();

		expect(draft.load()).toBeNull();
		expect(draft.consumeRestoreToast()).toBe(false);
	});
});

describe('DraftStore.scheduleSave', () => {
	beforeEach(() => {
		localStorage.clear();
		vi.useFakeTimers();
	});

	it('debounces writes and persists the latest values with the current schema version', () => {
		const draft = new DraftStore();
		const values = sampleValues();

		draft.scheduleSave(values);
		draft.scheduleSave({ ...values, name: 'web-10' });
		vi.advanceTimersByTime(1000);

		const raw = localStorage.getItem(DRAFT_STORAGE_KEY);
		expect(raw).not.toBeNull();
		const stored = JSON.parse(raw ?? '{}') as { schemaVersion: number; values: DraftValues };
		expect(stored.schemaVersion).toBe(DRAFT_SCHEMA_VERSION);
		expect(stored.values.name).toBe('web-10');
		vi.useRealTimers();
	});
});

describe('DraftStore.clear', () => {
	beforeEach(() => localStorage.clear());

	it('removes the draft immediately (FR-021: before any navigation)', () => {
		seedDraft(DRAFT_SCHEMA_VERSION);
		const draft = new DraftStore();

		draft.clear();

		expect(localStorage.getItem(DRAFT_STORAGE_KEY)).toBeNull();
	});
});
