import { describe, expect, it } from 'vitest';
import type { VmDisk } from '../detail.svelte';
import { diskSizeCeiling } from './disk-limit';

const disk = (key: string, sizeGB: number): VmDisk => ({ key, bus: 'scsi', busIndex: 0, storage: 'local-lvm', sizeGB, isBoot: false });

describe('diskSizeCeiling', () => {
	const disks = [disk('scsi0', 300), disk('scsi1', 150)];

	it('leaves a new disk the room the other disks do not use', () => {
		expect(diskSizeCeiling(500, disks)).toBe(50);
	});

	it('does not count the disk being resized', () => {
		expect(diskSizeCeiling(500, disks, 'scsi1')).toBe(200);
	});

	it('never goes below zero', () => {
		expect(diskSizeCeiling(400, disks)).toBe(0);
	});

	it('keeps the fallback ceiling when no cap is configured', () => {
		expect(diskSizeCeiling(0, disks)).toBe(2048);
		expect(diskSizeCeiling(undefined, disks)).toBe(2048);
	});
});
