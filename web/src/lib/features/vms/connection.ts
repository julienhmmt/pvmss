import type { VmNetworkInterface } from './detail.svelte';

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
