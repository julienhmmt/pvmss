<script lang="ts">
	/**
	 * The "needs attention" block at the top of the admin dashboard. The server
	 * computes the alerts; each row links to the page where the cause is fixed.
	 * With nothing to act on it collapses to one reassuring line.
	 */
	import { resolve } from '$app/paths';
	import Card from '$lib/shared/ui/Card.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { alertHref, alertMessage, type DashboardAlert } from './dashboard-alerts';

	interface Props {
		alerts: DashboardAlert[];
	}

	let { alerts }: Props = $props();
</script>

<Card pad="none" title={m['admin.dashboard.attentionTitle']()} titleId="dashboard-attention-heading">
	{#if alerts.length === 0}
		<p class="flex items-center gap-2 px-5 py-4 text-sm text-muted-foreground" data-testid="dashboard-all-clear">
			<span class="h-2 w-2 shrink-0 rounded-full bg-success" aria-hidden="true"></span>
			{m['admin.dashboard.allClear']()}
		</p>
	{:else}
		<ul class="divide-y divide-border" aria-labelledby="dashboard-attention-heading">
			{#each alerts as alert (`${alert.kind}/${alert.cluster}/${alert.subject ?? ''}`)}
				{@const target = alertHref(alert)}
				<li>
					<a
						href={typeof target === 'string' ? resolve(target) : resolve(target.route, target.params)}
						class="flex items-center gap-3 px-5 py-3 text-sm hover:bg-muted/60 pv-focus"
						data-testid="dashboard-alert"
						data-kind={alert.kind}
						data-severity={alert.severity}
					>
						<span
							class="h-2 w-2 shrink-0 rounded-full {alert.severity === 'critical' ? 'bg-destructive' : 'bg-warning'}"
							aria-hidden="true"
						></span>
						<span class="sr-only">
							{alert.severity === 'critical' ? m['admin.dashboard.severityCritical']() : m['admin.dashboard.severityWarning']()}:
						</span>
						<span class="min-w-0 flex-1 font-medium text-foreground">{alertMessage(alert)}</span>
						{#if alert.kind !== 'cluster_unreachable'}
							<span class="shrink-0 text-xs text-muted-foreground">{alert.cluster}</span>
						{/if}
						<span class="shrink-0 text-muted-foreground" aria-hidden="true">→</span>
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</Card>
