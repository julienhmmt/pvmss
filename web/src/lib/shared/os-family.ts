/**
 * The OS families PVMSS can actually distinguish.
 *
 * Proxmox reports a kernel family in `ostype`, not a distribution: `l26` is
 * any Linux 2.6+ guest, `win11` any modern Windows. That is the finest true
 * distinction the data carries, so nothing here tries to infer Ubuntu from
 * Debian - an unrecognised or absent value is `other`, never a guess.
 */
export type OsFamily = 'linux' | 'windows' | 'other';

const LINUX_OSTYPES: ReadonlySet<string> = new Set(['l24', 'l26']);

/** Proxmox's Windows families - not all are "win"-prefixed (w2k8, wvista). */
const WINDOWS_OSTYPES: ReadonlySet<string> = new Set([
	'wxp',
	'w2k',
	'w2k3',
	'w2k8',
	'wvista',
	'win7',
	'win8',
	'win10',
	'win11'
]);

export function osFamily(ostype: string): OsFamily {
	const value = ostype.trim().toLowerCase();
	if (LINUX_OSTYPES.has(value)) return 'linux';
	if (WINDOWS_OSTYPES.has(value)) return 'windows';
	return 'other';
}
