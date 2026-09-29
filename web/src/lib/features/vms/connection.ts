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

/**
 * What the Connect tab's SSH card can honestly say. "installing" wins over an
 * address: a live installer reports one, but there is no installed OS to SSH
 * into yet. PVMSS never probes port 22 (the address is guest-reported, so a
 * probe would be an SSRF vector) - readiness is the ISO, agent and address.
 */
export type ConnectState = 'stopped' | 'installing' | 'agentDisabled' | 'agentUnreachable' | 'noAddress' | 'ready';

interface ConnectInput {
	networkInterfaces?: readonly VmNetworkInterface[];
	guestAgent?: 'ok' | 'disabled' | 'unreachable';
	cdrom?: VmCdrom;
}

export function connectState(status: MachineDisplayStatus, entity: ConnectInput): ConnectState {
	if (status !== 'running') return 'stopped';
	if (entity.cdrom?.state === 'mounted') return 'installing';
	if (primaryAddress(entity.networkInterfaces)) return 'ready';
	if (entity.guestAgent === 'disabled') return 'agentDisabled';
	if (entity.guestAgent === 'unreachable') return 'agentUnreachable';
	return 'noAddress';
}
