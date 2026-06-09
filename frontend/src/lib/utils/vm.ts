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
 * VM list utilities (status badges, uptime formatting).
 */
export const vmList = {
	statusClass,
	uptimeLabel,
} as const;
