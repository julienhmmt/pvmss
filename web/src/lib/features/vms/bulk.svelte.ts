import { getContext, setContext } from 'svelte';
import { SvelteMap } from 'svelte/reactivity';
import { post } from '$lib/shared/api/client';
import type { VmListItem } from './list.svelte';

/**
 * Composite identity for a bulk-action target. Never a bare vmid — the same
 * VMID can exist in multiple clusters (T15), so selection and request bodies
 * always carry the cluster alongside.
 */
export interface BulkTarget {
	cluster: string;
	vmid: number;
}

/** One entry in the bulk-action response. */
export interface BulkTargetResult {
	cluster: string;
	vmid: number;
	status: 'ok' | 'error';
	message?: string;
}

/** The full POST /api/v1/vms/bulk-action response. */
export interface BulkActionResult {
	results: BulkTargetResult[];
}

/** Result summary counts for UI display. */
export interface BulkResultSummary {
	ok: number;
	error: number;
	total: number;
}

const BULK_ACTION_PATH = '/api/v1/vms/bulk-action';

function targetKey(cluster: string, vmid: number): string {
	return `${cluster}:${vmid}`;
}

function vmItemKey(item: VmListItem): string {
	return targetKey(item.cluster, item.vmid);
}

function toItemTarget(item: VmListItem): BulkTarget {
	return { cluster: item.cluster, vmid: item.vmid };
}

/**
 * Page-local selection state for the VM list (T17). Selection is keyed by
 * the composite (cluster, vmid) identity — never a bare vmid. The store is
 * NOT persisted in the URL: navigating to another page does not silently
 * carry selection over (quickstart SC-002). The store owns:
 *
 * - the selected set (toggle, selectPage, clearPage, clear),
 * - the bulk-action submission lifecycle (submitting, lastResult),
 * - per-target result lookup for the mixed-outcome panel (US2).
 */
export class VmBulkSelection {
	#selected = new SvelteMap<string, BulkTarget>();
	submitting = $state(false);
	lastResult = $state<BulkActionResult | null>(null);

	get selectedCount(): number {
		return this.#selected.size;
	}

	get hasSelection(): boolean {
		return this.#selected.size > 0;
	}

	get selectedTargets(): BulkTarget[] {
		return Array.from(this.#selected.values());
	}

	isSelected(cluster: string, vmid: number): boolean {
		return this.#selected.has(targetKey(cluster, vmid));
	}

	toggle(target: BulkTarget): void {
		const key = targetKey(target.cluster, target.vmid);
		if (this.#selected.has(key)) {
			this.#selected.delete(key);
		} else {
			this.#selected.set(key, target);
		}
	}

	selectPage(items: VmListItem[]): void {
		for (const item of items) {
			const key = vmItemKey(item);
			if (!this.#selected.has(key)) {
				this.#selected.set(key, toItemTarget(item));
			}
		}
	}

	clearPage(items: VmListItem[]): void {
		for (const item of items) {
			this.#selected.delete(vmItemKey(item));
		}
	}

	clear(): void {
		this.#selected.clear();
	}

	pageAllSelected(items: VmListItem[]): boolean {
		if (items.length === 0) return false;
		return items.every((item) => this.#selected.has(vmItemKey(item)));
	}

	clearResult(): void {
		this.lastResult = null;
	}

	resultForTarget(cluster: string, vmid: number): BulkTargetResult | undefined {
		if (this.lastResult === null) return undefined;
		return this.lastResult.results.find(
			(r) => r.cluster === cluster && r.vmid === vmid
		);
	}

	get resultSummary(): BulkResultSummary {
		if (this.lastResult === null) {
			return { ok: 0, error: 0, total: 0 };
		}
		let ok = 0;
		let error = 0;
		for (const r of this.lastResult.results) {
			if (r.status === 'ok') ok++;
			else error++;
		}
		return { ok, error, total: this.lastResult.results.length };
	}

	async submitBulkAction(action: string): Promise<BulkActionResult> {
		this.submitting = true;
		this.lastResult = null;
		try {
			const result = await post<BulkActionResult>(BULK_ACTION_PATH, {
				action,
				targets: this.selectedTargets
			});
			this.lastResult = result;
			return result;
		} finally {
			this.submitting = false;
		}
	}
}

const VM_BULK_CONTEXT_KEY = Symbol('vm-bulk');

export function setVmBulkContext(): VmBulkSelection {
	const selection = new VmBulkSelection();
	setContext(VM_BULK_CONTEXT_KEY, selection);
	return selection;
}

export function getVmBulkContext(): VmBulkSelection {
	return getContext<VmBulkSelection>(VM_BULK_CONTEXT_KEY);
}
