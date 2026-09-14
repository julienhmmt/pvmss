import { describe, it, expect } from 'vitest';
import {
	displayStatus,
	canConnect,
	isUncertainOutcome,
	type MachineDisplayInput,
	type TaskTraySnapshot,
	type TaskOutcomeLedger
} from './display-status';
import type { TrackedTask } from '$lib/features/tasks/tasks.svelte';

const vm = (overrides: Partial<MachineDisplayInput> = {}): MachineDisplayInput => ({
	cluster: 'default',
	vmid: 100,
	status: 'running',
	...overrides
});

const tray = (tasks: readonly TrackedTask[]): TaskTraySnapshot => ({ tasks });

const createTask = (overrides: Partial<TrackedTask> = {}): TrackedTask => ({
	upid: 'UPID:1',
	kind: 'vm_create',
	vmid: 100,
	name: 'web-01',
	cluster: 'default',
	deadline: Date.now() + 60_000,
	...overrides
});

const ledger = (entries: ReadonlyMap<string, 'failed' | 'partial'>): TaskOutcomeLedger => ({
	get(cluster: string, vmid: number): 'failed' | 'partial' | undefined {
		return entries.get(`${cluster}:${vmid}`);
	}
});

describe('displayStatus', () => {
	it('falls back to the server status when no signals are present', () => {
		expect(displayStatus(vm({ status: 'running' }), { tray: tray([]) })).toBe('running');
		expect(displayStatus(vm({ status: 'stopped' }), { tray: tray([]) })).toBe('stopped');
	});

	it('collapses the legacy paused status to stopped', () => {
		// paused is not a Calm-workspace display state; never produce an
		// invalid union member.
		expect(displayStatus(vm({ status: 'paused' }), { tray: tray([]) })).toBe('stopped');
	});

	it('derives starting from an in-flight start action', () => {
		expect(
			displayStatus(vm({ status: 'stopped' }), { tray: tray([]), inFlightAction: 'start' })
		).toBe('starting');
	});

	it('derives starting from an in-flight reboot action', () => {
		expect(
			displayStatus(vm({ status: 'running' }), { tray: tray([]), inFlightAction: 'reboot' })
		).toBe('starting');
	});

	it('derives stopping from an in-flight shutdown action', () => {
		expect(
			displayStatus(vm({ status: 'running' }), { tray: tray([]), inFlightAction: 'shutdown' })
		).toBe('stopping');
	});

	it('derives stopping from an in-flight stop action', () => {
		expect(
			displayStatus(vm({ status: 'running' }), { tray: tray([]), inFlightAction: 'stop' })
		).toBe('stopping');
	});

	it('derives provisioning from a running vm_create task', () => {
		const task = createTask();
		expect(displayStatus(vm({ status: 'stopped' }), { tray: tray([task]) })).toBe('provisioning');
	});

	it('ignores a vm_create task for a different VM', () => {
		const task = createTask({ vmid: 999 });
		expect(displayStatus(vm({ vmid: 100, status: 'stopped' }), { tray: tray([task]) })).toBe(
			'stopped'
		);
	});

	it('derives failed from a recorded failed outcome', () => {
		const map = new Map([['default:100', 'failed' as const]]);
		expect(
			displayStatus(vm({ status: 'stopped' }), { tray: tray([]), ledger: ledger(map) })
		).toBe('failed');
	});

	it('derives partial from a recorded partial outcome when the VM is not running', () => {
		const map = new Map([['default:100', 'partial' as const]]);
		expect(
			displayStatus(vm({ status: 'stopped' }), { tray: tray([]), ledger: ledger(map) })
		).toBe('partial');
	});

	it('does not show partial when the VM has since started running', () => {
		// partial is a creation-time signal; once the VM is up, the user can
		// reconfigure it, so the badge returns to running.
		const map = new Map([['default:100', 'partial' as const]]);
		expect(
			displayStatus(vm({ status: 'running' }), { tray: tray([]), ledger: ledger(map) })
		).toBe('running');
	});

	it('does not show failed when the VM has since started running', () => {
		// A `failed` outcome can come from a deadline / 404 / poll-error, not
		// necessarily a true creation failure. If the server reports the VM
		// is up, the user can connect - never contradict a live VM with
		// "Creation failed".
		const map = new Map([['default:100', 'failed' as const]]);
		expect(
			displayStatus(vm({ status: 'running' }), { tray: tray([]), ledger: ledger(map) })
		).toBe('running');
	});

	it('prioritizes an in-flight power action over a provisioning task', () => {
		// a vm_create task may still be in the tray while the user fires a
		// start; the live transition wins.
		const task = createTask();
		expect(
			displayStatus(vm({ status: 'stopped' }), {
				tray: tray([task]),
				inFlightAction: 'start'
			})
		).toBe('starting');
	});
});

describe('canConnect', () => {
	it('is true only for a running VM with a reported address', () => {
		expect(canConnect('running', '10.0.0.5')).toBe(true);
	});

	it('is false for a running VM with no address (never fabricate)', () => {
		expect(canConnect('running', undefined)).toBe(false);
		expect(canConnect('running', null)).toBe(false);
		expect(canConnect('running', '')).toBe(false);
	});

	it('is false for a non-running VM even with an address', () => {
		expect(canConnect('stopped', '10.0.0.5')).toBe(false);
		expect(canConnect('provisioning', '10.0.0.5')).toBe(false);
	});
});

describe('isUncertainOutcome', () => {
	it('flags failed and partial as uncertain (no duplicate creation)', () => {
		expect(isUncertainOutcome('failed')).toBe(true);
		expect(isUncertainOutcome('partial')).toBe(true);
	});

	it('does not flag settled states', () => {
		expect(isUncertainOutcome('running')).toBe(false);
		expect(isUncertainOutcome('stopped')).toBe(false);
		expect(isUncertainOutcome('provisioning')).toBe(false);
		expect(isUncertainOutcome('starting')).toBe(false);
		expect(isUncertainOutcome('stopping')).toBe(false);
	});
});
