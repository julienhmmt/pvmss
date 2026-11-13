// Admin node detail helpers: tab parsing, workload summary and the health
// auto-poll. Pure functions and a small interval wrapper live here so the
// route page stays a thin render layer and the logic is vitest-able.

export type NodeTab = 'health' | 'workloads' | 'hardware';

const NODE_TABS: ReadonlySet<NodeTab> = new Set(['workloads', 'hardware']);

/** Maps the ?tab= query value to a tab; absent or unknown values land on health. */
export function parseNodeTab(raw: string | null): NodeTab {
	if (raw === 'workloads' || raw === 'hardware') return raw;
	return 'health';
}

/** Serializes a tab back to the query value; the default tab keeps URLs clean. */
export function nodeTabParam(tab: NodeTab): string | null {
	return NODE_TABS.has(tab) ? tab : null;
}

const STATUS_ORDER: readonly string[] = ['running', 'stopped', 'paused'];

/**
 * Counts workload statuses for the health summary line: known statuses first
 * in a fixed order, anything else alphabetically after them.
 */
export function workloadStatusCounts(statuses: string[]): [status: string, count: number][] {
	const counts = new Map<string, number>();
	for (const status of statuses) counts.set(status, (counts.get(status) ?? 0) + 1);
	const known = STATUS_ORDER.filter((status) => counts.has(status)).map(
		(status) => [status, counts.get(status) ?? 0] as [string, number]
	);
	const rest = [...counts.entries()]
		.filter(([status]) => !STATUS_ORDER.includes(status))
		.sort(([a], [b]) => a.localeCompare(b));
	return [...known, ...rest];
}

export const HEALTH_POLL_INTERVAL_MS = 20_000;

export interface HealthPoll {
	start(): void;
	stop(): void;
}

/**
 * Re-runs `run` every `intervalMs` while started, skipping ticks where the
 * document is hidden so the page stops hitting Proxmox in background tabs.
 * `start` is idempotent; `stop` cancels pending ticks.
 */
export function createHealthPoll(run: () => void, intervalMs: number = HEALTH_POLL_INTERVAL_MS): HealthPoll {
	let timer: ReturnType<typeof setInterval> | null = null;
	const tick = (): void => {
		if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return;
		run();
	};
	return {
		start() {
			if (timer !== null) return;
			timer = setInterval(tick, intervalMs);
		},
		stop() {
			if (timer === null) return;
			clearInterval(timer);
			timer = null;
		}
	};
}
