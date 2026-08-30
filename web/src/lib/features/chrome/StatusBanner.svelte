<script lang="ts">
	import { getStatusContext, type Severity } from './status.svelte';
	import { m } from '$lib/paraglide/messages.js';

	/**
	 * StatusBanner — one component, three severities (info / degraded /
	 * unhealthy), plus an "unknown" variant for poll failures. An aria-live
	 * region announces severity changes (constitution XII). Renders nothing
	 * when severity is "none" (SC-004 baseline).
	 */
	const status = getStatusContext();

	const SEVERITY_STYLES: Record<Exclude<Severity, 'none'>, string> = {
		info: 'bg-info-soft text-info-soft-foreground border-info-soft-border',
		degraded: 'bg-warning-soft text-warning-soft-foreground border-warning-soft-border',
		unhealthy: 'bg-destructive-soft text-destructive-soft-foreground border-destructive-soft-border',
		unknown: 'bg-muted text-muted-foreground border-border'
	};

	function message(): string {
		switch (status.severity) {
			case 'info':
				return m['banner.info.demo']();
			case 'degraded':
				return m['banner.degraded.clusters']();
			case 'unhealthy':
				return m['banner.unhealthy.service']();
			case 'unknown':
				return m['banner.unknown.poll']();
			default:
				return '';
		}
	}
</script>

{#if status.severity !== 'none'}
	<div
		role="status"
		aria-live="polite"
		class="border-b px-4 py-2 text-center text-sm {SEVERITY_STYLES[status.severity]}"
	>
		<span class="inline-flex items-center gap-2">
			{message()}
			{#if status.clusterDownCounts}
				<span class="font-medium">
					{m['banner.degraded.clustersDetail'](status.clusterDownCounts)}
				</span>
			{/if}
		</span>
		{#if status.severity === 'degraded'}
			<button
				type="button"
				class="ml-3 underline underline-offset-2 hover:no-underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
				onclick={() => void status.retryClusterConnection()}
				data-testid="banner-cluster-retry"
			>
				{m['banner.degraded.retry']()}
			</button>
		{/if}
	</div>
{/if}
