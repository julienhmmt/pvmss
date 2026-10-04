import type { VmCdrom, VmNetworkInterface } from './detail.svelte';
import type { MachineDisplayStatus } from './display-status';

/** An address PVMSS can offer for SSH, with the NIC it came from. */
export interface MachineAddress {
	address: string;
	bridge: string;
}

function isUsableIPv4(ip: string): boolean {
	if (!/^\d{1,3}(\.\d{1,3}){3}$/.test(ip)) return false;
	return !ip.startsWith('127.') && !ip.startsWith('169.254.');
}

function isUsableIPv6(ip: string): boolean {
	const lower = ip.toLowerCase();
	return lower.includes(':') && lower !== '::1' && !lower.startsWith('fe80:');
}

/**
 * The address to show in the SSH command: the first routable IPv4 the guest
 * agent reported, else the first global IPv6. Returns null when the machine
 * reported nothing usable - the caller must then say "not available yet"
 * and never fabricate one (DESIGN.md §7, "No guessed addresses").
 */
export function primaryAddress(interfaces: readonly VmNetworkInterface[] | undefined): MachineAddress | null {
	const nics = interfaces ?? [];
	for (const nic of nics) {
		const ip = nic.ipAddresses.map((value) => value.split('/')[0] ?? '').find(isUsableIPv4);
		if (ip) return { address: ip, bridge: nic.bridge };
	}
	for (const nic of nics) {
		const ip = nic.ipAddresses.map((value) => value.split('/')[0] ?? '').find(isUsableIPv6);
		if (ip) return { address: ip, bridge: nic.bridge };
	}
	return null;
}

/** The SSH command line. The user is a placeholder when PVMSS does not know it. */
export function sshCommand(user: string | null, address: string): string {
	const host = address.includes(':') ? `[${address}]` : address;
	return `ssh ${user && user.trim() !== '' ? user.trim() : 'USER'}@${host}`;
}

/** Proxmox Windows ostypes all start with "w" (w11, w2k22, win10, wxp...). */
export function isWindowsOs(ostype: string | undefined): boolean {
	return (ostype ?? '').toLowerCase().startsWith('w');
}

/** The key Proxmox assigns to the VM's fixed CD-ROM drive in boot=order=. */
const CDROM_BOOT_KEY = 'ide2';

/** True while the boot order says the next boot (and therefore the running
 * installer) came from the mounted ISO. Empty/unknown order defers to the
 * mounted check: that was the only signal before boot order was reported. */
function bootsFromIso(bootOrder: readonly string[] | undefined): boolean {
	if (!bootOrder || bootOrder.length === 0) return true;
	return bootOrder[0] === CDROM_BOOT_KEY;
}

/**
 * What the Connect tab's connect card (SSH, or RDP on Windows guests) can
 * honestly say. "installing" wins over an
 * address: a live installer reports one, but there is no installed OS to
 * reach yet. PVMSS never probes ports (the address is guest-reported, so a
 * probe would be an SSRF vector) - readiness is the ISO, agent and address.
 */
export type ConnectState = 'stopped' | 'installing' | 'agentDisabled' | 'agentUnreachable' | 'noAddress' | 'ready';

interface ConnectInput {
	networkInterfaces?: readonly VmNetworkInterface[];
	guestAgent?: 'ok' | 'disabled' | 'unreachable';
	cdrom?: VmCdrom;
	bootOrder?: readonly string[];
}

export function connectState(status: MachineDisplayStatus, entity: ConnectInput): ConnectState {
	if (status !== 'running') return 'stopped';
	// A mounted ISO only means "installing" while the machine actually boots
	// from it: once the boot order leads with a disk the install is done and
	// the attached ISO is just media in the drive.
	if (entity.cdrom?.state === 'mounted' && bootsFromIso(entity.bootOrder)) return 'installing';
	if (primaryAddress(entity.networkInterfaces)) return 'ready';
	if (entity.guestAgent === 'disabled') return 'agentDisabled';
	if (entity.guestAgent === 'unreachable') return 'agentUnreachable';
	return 'noAddress';
}
