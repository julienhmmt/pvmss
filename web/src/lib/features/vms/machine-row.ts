import { m } from '$lib/paraglide/messages.js';
import type { MachineDisplayStatus } from './display-status';

/** The explanation line under a row, when its state needs one. */
export interface RowHint {
	text: string;
	/** Failed and partial read as errors; the rest are quiet. */
	tone: 'muted' | 'error';
}

export function rowHint(status: MachineDisplayStatus): RowHint | null {
	switch (status) {
		case 'provisioning':
			return { text: m['vms.list.hint.provisioning'](), tone: 'muted' };
		case 'starting':
			return { text: m['vms.list.hint.starting'](), tone: 'muted' };
		case 'stopping':
			return { text: m['vms.list.hint.stopping'](), tone: 'muted' };
		case 'stopped':
			return { text: m['vms.list.hint.stopped'](), tone: 'muted' };
		case 'failed':
			return { text: m['vms.list.hint.failed'](), tone: 'error' };
		case 'partial':
			return { text: m['vms.list.hint.partial'](), tone: 'error' };
		default:
			return null;
	}
}

/**
 * The one labelled action a row carries. A stopped machine offers "Start";
 * everything else opens the detail page. The list DTO has no address, so a
 * row never claims "Connect" (DESIGN.md §7, "No readiness claim without
 * connection data") - the detail page's Connect tab decides that.
 */
export function rowAction(status: MachineDisplayStatus): 'start' | 'details' {
	return status === 'stopped' ? 'start' : 'details';
}

/** "8 GiB", "1.5 GiB", "512 MiB" - the list's resource line has no room for
 *  formatBytes' fixed decimal. */
export function compactBytes(bytes: number): string {
	const gib = bytes / 1024 ** 3;
	if (gib >= 1) return `${Number.isInteger(gib) ? gib : gib.toFixed(1)} GiB`;
	return `${Math.round(bytes / 1024 ** 2)} MiB`;
}
