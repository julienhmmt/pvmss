<script lang="ts">
	import { bannerSeverity, getStatusContext, type Severity } from './status.svelte';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * StatusBanner - one component, three severities (info / degraded /
	 * unhealthy), plus an "unknown" variant for poll failures. An aria-live
	 * region announces severity changes (constitution XII). Renders nothing
	 * when severity is "none" (SC-004 baseline). The "degraded" cluster
	 * banner is admin-only (bannerSeverity); visitors and non-admin users
	 * get the demo-mode "info" instead.
	 */
	const status = getStatusContext();
	const session = getSessionContext();
	const severity: Severity = $derived(
		bannerSeverity({ severity: status.severity, isAdmin: session.isAdmin, demoMode: status.raw?.demoMode ?? false })
	);

	const SEVERITY_STYLES: Record<Exclude<Severity, 'none'>, string> = {
		info: 'bg-info-soft text-info-soft-foreground border-info-soft-border',
		degraded: 'bg-warning-soft text-warning-soft-foreground border-warning-soft-border',
		unhealthy: 'bg-destructive-soft text-destructive-soft-foreground border-destructive-soft-border',
		unknown: 'bg-muted text-muted-foreground border-border'
	};

	function message(): string {
		switch (severity) {
			case 'info':
				return m['banner.info.demo']();
			case 'degraded':
				return status.ownClusterDown ? m['banner.degraded.ownCluster']() : m['banner.degraded.clusters']();
			case 'unhealthy':
				return m['banner.unhealthy.service']();
			case 'unknown':
				return m['banner.unknown.poll']();
			default:
				return '';
		}
	}
</script>

{#if severity !== 'none'}
	<div
		role="status"
		aria-live="polite"
		class="border-b px-4 py-2 text-center text-sm {SEVERITY_STYLES[severity]}"
	>
		<span class="inline-flex flex-wrap items-center justify-center gap-x-2">
			{message()}
			{#if severity === 'degraded' && status.clusterDownCounts && !status.ownClusterDown}
				<span class="font-medium">
					{m['banner.degraded.clustersDetail'](status.clusterDownCounts)}
				</span>
			{/if}
		</span>
		{#if severity === 'degraded'}
			<button
				type="button"
				class="ml-3 underline underline-offset-2 hover:no-underline pv-focus"
				onclick={() => void status.retryClusterConnection()}
				data-testid="banner-cluster-retry"
			>
				{m['banner.degraded.retry']()}
			</button>
		{/if}
	</div>
{/if}
