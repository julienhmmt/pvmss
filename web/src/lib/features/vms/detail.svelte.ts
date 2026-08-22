import { getContext, setContext } from 'svelte';
import { get, post, del, patch, put, ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import type { VmStatus } from './list.svelte';

export type VmAction = 'start' | 'stop' | 'shutdown' | 'reboot' | 'reset';

export interface VmDetailEntity {
	cluster: string;
	vmid: number;
	name: string;
	node: string;
	pool: string;
	status: VmStatus;
	tags: string[];
	cpuCores: number;
	memoryTotal: number;
	diskTotal: number;
	uptimeSeconds?: number;
	description?: string;
	sockets?: number;
	cores?: number;
	disks?: VmDisk[];
	cdrom?: VmCdrom;
	networkInterfaces?: VmNetworkInterface[];
	hasSerial?: boolean;
}

export interface VmDisk {
	key: string;
	bus: 'virtio' | 'scsi' | 'sata' | 'ide';
	busIndex: number;
	storage: string;
	sizeGB: number;
	isBoot: boolean;
}

export interface VmCdrom {
	state: 'absent' | 'empty' | 'mounted';
	isoVolId?: string;
}

export interface VmNetworkInterface {
	index: number;
	bridge: string;
	model: string;
	mac: string;
	vlan: number | null;
	rateMbps: number | null;
	ipAddresses: string[];
}

export interface HardwareOptions {
	storages: { node: string; storage: string; type: string }[];
	bridges: { node: string; bridge: string }[];
	isos: { volId: string; node: string; storage: string; name: string; sizeBytes: number }[];
	limits: {
		maxSockets: number;
		maxCores: number;
		maxMemoryMB: number;
		maxDiskPerVMGB: number;
		maxNetworkCards: number;
		remainingBusSlots: Record<string, number>;
	};
}

interface ActionResponse {
	status: string;
}

interface DeleteResponse {
	status: string;
}

/**
 * State for the single-VM detail view (V15). One store instance per consuming
 * screen (constitution VII: no module singletons). The entity is `$state.raw`
 * because it is replaced wholesale on load/reload, not mutated field-by-field
 * — except for the optimistic status flip during a power action (V12), which
 * is a deliberate local mutation reconciled by reload().
 */
export class VmDetailStore {
	readonly cluster: string;
	readonly vmid: number;

	entity = $state.raw<VmDetailEntity | null>(null);
	loading = $state.raw(false);
	error = $state.raw<string | null>(null);

	/** True while a power action is in flight; the UI shows an optimistic status. */
	actionInFlight = $state.raw(false);
	actionError = $state.raw<string | null>(null);

	/** True while a delete is in flight; the UI disables the button. */
	deleteInFlight = $state.raw(false);
	deleteError = $state.raw<string | null>(null);
	/** Stable error code from the last delete attempt (e.g. "vm_running") so the
	 * dialog can branch on it without string-matching the message. */
	deleteErrorCode = $state.raw<string | null>(null);

	/** True while a patch (rename/description) is in flight. */
	patchInFlight = $state.raw(false);
	patchError = $state.raw<string | null>(null);

	hardwareOptions = $state.raw<HardwareOptions | null>(null);
	hardwareLoading = $state.raw(false);
	hardwareError = $state.raw<string | null>(null);
	diskInFlight = $state.raw(false);
	diskError = $state.raw<string | null>(null);
	cdromInFlight = $state.raw(false);
	networkInFlight = $state.raw(false);
	hardwareInFlight = $state.raw(false);
	writeError = $state.raw<string | null>(null);

	/** True while the serial-console retrofit is in flight. */
	serialEnabling = $state.raw(false);
	serialEnableError = $state.raw<string | null>(null);

	/** Retrofits a serial port (serial0) onto an existing VM so the Text
	 * console works, then reloads so the entity's hasSerial flips. */
	async enableSerialConsole(): Promise<boolean> {
		if (this.serialEnabling || this.entity === null) return false;
		this.serialEnabling = true;
		this.serialEnableError = null;
		try {
			await post<VmDetailEntity>(`${this.#basePath}/serial`, {});
			await this.load();
			return true;
		} catch (err) {
			this.serialEnableError = errorMessage(err, () => m['vms.console.serial.enableError']());
			return false;
		} finally {
			this.serialEnabling = false;
		}
	}

	/** Set after a successful delete so the page can navigate away. */
	deleted = $state.raw(false);

	#basePath: string;

	constructor(cluster: string, vmid: number) {
		this.cluster = cluster;
		this.vmid = vmid;
		this.#basePath = `/api/v1/vms/${encodeURIComponent(cluster)}/${vmid}`;
	}

	async load(): Promise<void> {
		this.loading = true;
		this.error = null;
		try {
			this.entity = await get<VmDetailEntity>(this.#basePath);
			if (this.hardwareOptions === null) await this.loadHardwareOptions();
		} catch (err) {
			this.error = errorMessage(err, () => m['vms.detail.errorLoading']());
		} finally {
			this.loading = false;
		}
	}

	async loadHardwareOptions(): Promise<void> {
		this.hardwareLoading = true;
		this.hardwareError = null;
		try {
			this.hardwareOptions = await get<HardwareOptions>(`${this.#basePath}/hardware-options`);
		} catch (err) {
			this.hardwareError = errorMessage(err, () => m['vms.detail.errorHardwareOptions']());
		} finally {
			this.hardwareLoading = false;
		}
	}

	async addDisk(bus: VmDisk['bus'], storage: string, sizeGB: number): Promise<boolean> {
		if (this.diskInFlight) return false;
		this.diskInFlight = true;
		this.diskError = null;
		try {
			await post<VmDisk>(`${this.#basePath}/disks`, { bus, storage, sizeGB });
			await this.load();
			return true;
		} catch (err) {
			this.diskError = errorMessage(err, () => m['vms.detail.errorAddDisk']());
			return false;
		} finally {
			this.diskInFlight = false;
		}
	}

	async resizeDisk(diskKey: string, sizeGB: number): Promise<boolean> {
		if (this.diskInFlight) return false;
		this.diskInFlight = true;
		this.diskError = null;
		try {
			await put<VmDisk>(`${this.#basePath}/disks/${encodeURIComponent(diskKey)}/resize`, { sizeGB });
			await this.load();
			return true;
		} catch (err) {
			this.diskError = errorMessage(err, () => m['vms.detail.errorResizeDisk']());
			return false;
		} finally {
			this.diskInFlight = false;
		}
	}

	async deleteDisk(diskKey: string): Promise<boolean> {
		if (this.diskInFlight) return false;
		this.diskInFlight = true;
		this.diskError = null;
		try {
			await del<DeleteResponse>(`${this.#basePath}/disks/${encodeURIComponent(diskKey)}`);
			await this.load();
			return true;
		} catch (err) {
			this.diskError = errorMessage(err, () => m['vms.detail.errorDeleteDisk']());
			return false;
		} finally {
			this.diskInFlight = false;
		}
	}

	async setCdrom(action: 'mount' | 'disconnect' | 'remove', isoVolId?: string): Promise<void> {
		if (this.cdromInFlight) return;
		this.cdromInFlight = true;
		this.writeError = null;
		try {
			await patch<VmCdrom>(`${this.#basePath}/cdrom`, { action, ...(isoVolId ? { isoVolId } : {}) });
			await this.load();
		} catch (err) {
			this.writeError = errorMessage(err, () => m['vms.detail.errorUpdateCdrom']());
		} finally {
			this.cdromInFlight = false;
		}
	}

	async updateNetwork(interfaces: Omit<VmNetworkInterface, 'mac' | 'ipAddresses'>[]): Promise<void> {
		if (this.networkInFlight) return;
		this.networkInFlight = true;
		this.writeError = null;
		try {
			await put<VmNetworkInterface[]>(`${this.#basePath}/network`, { interfaces });
			await this.load();
		} catch (err) {
			this.writeError = errorMessage(err, () => m['vms.detail.errorUpdateNetwork']());
		} finally {
			this.networkInFlight = false;
		}
	}

	async updateHardware(patch: { sockets?: number; cores?: number; memoryMB?: number; tags?: string[] }): Promise<void> {
		if (this.hardwareInFlight) return;
		this.hardwareInFlight = true;
		this.writeError = null;
		try {
			await put<VmDetailEntity>(`${this.#basePath}/hardware`, patch);
			await this.load();
		} catch (err) {
			this.writeError = errorMessage(err, () => m['vms.detail.errorUpdateHardware']());
		} finally {
			this.hardwareInFlight = false;
		}
	}

	/**
	 * Triggers a power action (V12). The status flips optimistically before the
	 * server responds, then reload() reconciles with the authoritative state.
	 * `aria-live` on the status element (constitution XII) announces the flip.
	 */
	async action(kind: VmAction): Promise<void> {
		if (this.actionInFlight || this.entity === null) return;
		this.actionError = null;
		this.actionInFlight = true;

		const previousStatus = this.entity.status;
		this.entity = { ...this.entity, status: optimisticStatus(kind) };

		try {
			await post<ActionResponse>(`${this.#basePath}/actions`, { action: kind });
			await this.load();
		} catch (err) {
			// Revert the optimistic flip on failure.
			if (this.entity !== null) {
				this.entity = { ...this.entity, status: previousStatus };
			}
			this.actionError = errorMessage(err, () => m['vms.detail.errorAction']());
		} finally {
			this.actionInFlight = false;
		}
	}

	/**
	 * Permanently deletes the VM (V14: no soft-delete, no undo). When force is
	 * true, the server force-stops a running VM before destroying it — the UI
	 * only sets this after the user confirms the force-stop in the delete dialog.
	 * A running VM without force is rejected with 409 (code "vm_running") so the
	 * dialog can prompt for confirmation.
	 */
	async delete(force = false): Promise<void> {
		if (this.deleteInFlight || this.entity === null) return;
		this.deleteError = null;
		this.deleteErrorCode = null;
		this.deleteInFlight = true;
		try {
			const path = force ? `${this.#basePath}?force=true` : this.#basePath;
			await del<DeleteResponse>(path);
			this.deleted = true;
		} catch (err) {
			this.deleteError = errorMessage(err, () => m['vms.detail.errorDelete']());
			this.deleteErrorCode = err instanceof ApiRequestError ? err.code : null;
		} finally {
			this.deleteInFlight = false;
		}
	}

	/**
	 * Renames and/or updates the description (V16/V17). Returns true on success
	 * so the caller can exit inline-edit mode. Empty values are omitted from the
	 * request body — the server treats an absent field as "no change".
	 */
	async patch(name: string | null, description: string | null): Promise<boolean> {
		if (this.patchInFlight || this.entity === null) return false;
		this.patchError = null;
		this.patchInFlight = true;
		try {
			const body: Record<string, string> = {};
			if (name !== null) body.name = name;
			if (description !== null) body.description = description;
			this.entity = await patch<VmDetailEntity>(this.#basePath, body);
			return true;
		} catch (err) {
			this.patchError = errorMessage(err, () => m['vms.detail.errorUpdate']());
			return false;
		} finally {
			this.patchInFlight = false;
		}
	}
}

/** optimisticStatus returns the status a VM is expected to show after kind. */
function optimisticStatus(kind: VmAction): VmStatus {
	switch (kind) {
		case 'start':
		case 'reboot':
		case 'reset':
			return 'running';
		case 'stop':
		case 'shutdown':
			return 'stopped';
	}
}

function errorMessage(err: unknown, fallback: () => string): string {
	return err instanceof ApiRequestError ? err.message : fallback();
}

const VM_DETAIL_CONTEXT_KEY = Symbol('vm-detail');

/** Called once, by the route that owns this state (constitution VII). */
export function setVmDetailContext(cluster: string, vmid: number): VmDetailStore {
	const store = new VmDetailStore(cluster, vmid);
	setContext(VM_DETAIL_CONTEXT_KEY, store);
	return store;
}

export function getVmDetailContext(): VmDetailStore {
	return getContext<VmDetailStore>(VM_DETAIL_CONTEXT_KEY);
}
