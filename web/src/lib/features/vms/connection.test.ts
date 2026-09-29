import { describe, expect, it } from 'vitest';
import { connectState, primaryAddress, sshCommand } from './connection';
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

describe('connectState', () => {
	const withIp = [nic('vmbr0', ['192.168.1.215'])];
	const base = { networkInterfaces: withIp, guestAgent: 'ok' as const, cdrom: { state: 'empty' as const } };

	it('is stopped whenever the machine is not running, ISO or not', () => {
		expect(connectState('stopped', { ...base, cdrom: { state: 'mounted' } })).toBe('stopped');
	});

	it('is installing while an ISO is mounted, even if the live installer reports an IP', () => {
		expect(connectState('running', { ...base, cdrom: { state: 'mounted', isoVolId: 'local:iso/a.iso' } })).toBe('installing');
	});

	it('maps the agent channel when no ISO is mounted', () => {
		expect(connectState('running', { ...base, guestAgent: 'disabled', networkInterfaces: [] })).toBe('agentDisabled');
		expect(connectState('running', { ...base, guestAgent: 'unreachable', networkInterfaces: [] })).toBe('agentUnreachable');
		expect(connectState('running', { ...base, networkInterfaces: [nic('vmbr0', [])] })).toBe('noAddress');
	});

	it('is ready only with a usable address', () => {
		expect(connectState('running', base)).toBe('ready');
	});

	it('treats an entity without cdrom info as having no ISO', () => {
		expect(connectState('running', { networkInterfaces: withIp, guestAgent: 'ok' })).toBe('ready');
	});
});
