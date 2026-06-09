/**
 * Shared VM helper functions for list views (home, profile, search).
 * Extracted in Stage 8 for consistency across VM listing pages.
 * Exported as a single object to satisfy the "one export per file" rule.
 */

const statusClass = (status: string): string => {
	if (status === 'running') return 'pv-badge--online';
	if (status === 'stopped') return 'pv-badge--offline';
	return 'pv-badge--warn';
};

const uptimeLabel = (seconds: number): string => {
	if (!seconds) return '—';
	const d = Math.floor(seconds / 86400);
	const h = Math.floor((seconds % 86400) / 3600);
	if (d > 0) return `${d}d ${h}h`;
	const m = Math.floor((seconds % 3600) / 60);
	return h > 0 ? `${h}h ${m}m` : `${m}m`;
};

/**
 * formatMem converts memory in MB to a compact human string (e.g. "4 GB", "512 MB").
 * Returns "—" for zero/undefined.
 */
const formatMem = (mb: number | undefined): string => {
	if (!mb || mb <= 0) return '—';
	const gb = mb / 1024;
	if (gb >= 1) {
		return `${gb.toFixed(gb >= 10 ? 0 : 1)} GB`;
	}
	return `${Math.round(mb)} MB`;
};

/**
 * formatDisk converts disk in MB to a compact human string.
 * Returns "—" for zero/undefined.
 */
const formatDisk = (mb: number | undefined): string => {
	if (!mb || mb <= 0) return '—';
	const gb = mb / 1024;
	if (gb >= 1) {
		return `${gb.toFixed(gb >= 10 ? 0 : 1)} GB`;
	}
	return `${Math.round(mb)} MB`;
};

/**
 * splitTags splits a semicolon-separated Proxmox tags string and removes
 * the internal "pvmss" tag and empties. Case-insensitive removal of pvmss.
 */
const splitTags = (tags: string | undefined): readonly string[] => {
	if (!tags) return [];
	return tags
		.split(';')
		.map((t) => t.trim())
		.filter((t) => t.length > 0 && t.toLowerCase() !== 'pvmss');
};

/**
 * VM list utilities (status badges, uptime formatting, resource formatting, tag utils).
 */
export const vmList = {
	statusClass,
	uptimeLabel,
	formatMem,
	formatDisk,
	splitTags,
} as const;
