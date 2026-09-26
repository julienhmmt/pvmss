import { describe, expect, it } from 'vitest';
import { primaryAddress, sshCommand } from './connection';
import type { VmNetworkInterface } from './detail.svelte';

function nic(bridge: string, ipAddresses: string[]): VmNetworkInterface {
	return { index: 0, bridge, model: 'virtio', mac: 'aa:bb', vlan: null, rateMbps: null, ipAddresses };
}

describe('primaryAddress', () => {
	it('prefers the first routable IPv4, skipping loopback and link-local', () => {
		expect(primaryAddress([nic('vmbr0', ['127.0.0.1', 'fe80::1', '169.254.3.4']), nic('vmbr1', ['10.0.0.5/24'])])).toEqual({
			address: '10.0.0.5',
			bridge: 'vmbr1'
		});
	});

	it('falls back to a global IPv6', () => {
		expect(primaryAddress([nic('vmbr0', ['fe80::1', '2001:db8::7'])])).toEqual({ address: '2001:db8::7', bridge: 'vmbr0' });
	});

	it('returns null rather than guessing', () => {
		expect(primaryAddress(undefined)).toBeNull();
		expect(primaryAddress([nic('vmbr0', [])])).toBeNull();
		expect(primaryAddress([nic('vmbr0', ['127.0.0.1', 'fe80::2'])])).toBeNull();
	});
});

describe('sshCommand', () => {
	it('uses the known user, a placeholder otherwise, and brackets IPv6', () => {
		expect(sshCommand('debian', '10.0.0.5')).toBe('ssh debian@10.0.0.5');
		expect(sshCommand(null, '10.0.0.5')).toBe('ssh USER@10.0.0.5');
		expect(sshCommand(' ', '2001:db8::7')).toBe('ssh USER@[2001:db8::7]');
	});
});
