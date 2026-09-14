/**
 * Tracks VMs the current tab just deleted, so the list view can hide them
 * even if a stale server response still reports them.
 *
 * Proxmox destroy is asynchronous (server/internal/cluster/proxmox_writer.go:
 * DELETE returns a task UPID that PVMSS discards without waiting on). The
 * server refreshes its inventory cache immediately after issuing the delete,
 * but the VM often isn't actually gone from Proxmox yet, so that refresh
 * still captures it — the cache does not clear until a later inventory tick
 * once the destroy task has actually finished. This client-side suppression
 * closes that visible gap.
 */

const STORAGE_KEY = 'pvmss:recently-deleted-vms';
const TTL_MS = 5 * 60 * 1000;

interface Entry {
	key: string;
	at: number;
}

function readEntries(): Entry[] {
	try {
		const raw = sessionStorage.getItem(STORAGE_KEY);
		if (raw === null) return [];
		const now = Date.now();
		return (JSON.parse(raw) as Entry[]).filter((e) => now - e.at < TTL_MS);
	} catch {
		return [];
	}
}

function writeEntries(entries: Entry[]): void {
	try {
		sessionStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
	} catch {
		// sessionStorage unavailable (private mode, disabled) — suppression just
		// won't survive the navigation; the VM still disappears on the next
		// inventory refresh.
	}
}

/** Marks a VM as just-deleted so the list view hides it for a short window. */
export function markVmDeleted(cluster: string, vmid: number): void {
	const entries = readEntries();
	entries.push({ key: `${cluster}:${vmid}`, at: Date.now() });
	writeEntries(entries);
}

/** True if the VM was deleted recently enough that it should stay hidden. */
export function isRecentlyDeletedVm(cluster: string, vmid: number): boolean {
	return readEntries().some((e) => e.key === `${cluster}:${vmid}`);
}
