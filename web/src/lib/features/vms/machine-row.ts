import { m } from '$lib/paraglide/messages.js';
import type { MachineDisplayStatus } from './display-status';

/** OsMark tones - the recognition anchor on each row (DESIGN.md §8). */
export type MachineTone = 'accent' | 'subtle' | 'success';

const TONES: readonly MachineTone[] = ['accent', 'subtle', 'success'];

/**
 * Two-letter mark for a machine. The list DTO carries no OS, so the mark is
 * derived from the machine's own name: a stable recognition anchor, not a
 * logo (DESIGN.md §8 "OS marks").
 */
export function machineInitials(name: string): string {
	const parts = name.trim().split(/[\s._-]+/).filter(Boolean);
	if (parts.length === 0) return '?';
	const first = parts[0] ?? '';
	const second = parts[1] ?? '';
	const letters = second !== '' && !/^\d+$/.test(second) ? `${first[0] ?? ''}${second[0] ?? ''}` : first.slice(0, 2);
	return letters.toUpperCase();
}

/** Deterministic tone per name, so a machine keeps its colour across reloads. */
export function machineTone(name: string): MachineTone {
	let hash = 0;
	for (const char of name) hash = (hash * 31 + (char.codePointAt(0) ?? 0)) >>> 0;
	return TONES[hash % TONES.length] ?? 'subtle';
}

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
