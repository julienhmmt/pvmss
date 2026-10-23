import type { VmDisk } from '../detail.svelte';

/** Input ceiling when the cluster sets no per-VM disk cap. */
const UNCAPPED_DISK_GB = 2048;

/**
 * Largest size one disk may take. The gabarit's maxDiskPerVMGB caps the VM's
 * total disk (the server checks the sum), so the other disks are subtracted;
 * exceptKey is the disk being resized.
 */
export function diskSizeCeiling(maxPerVMGB: number | undefined, disks: readonly VmDisk[] = [], exceptKey?: string): number {
	if (maxPerVMGB === undefined || maxPerVMGB <= 0) return UNCAPPED_DISK_GB;

	const others = disks.filter((disk) => disk.key !== exceptKey).reduce((sum, disk) => sum + disk.sizeGB, 0);

	return Math.max(maxPerVMGB - others, 0);
}
