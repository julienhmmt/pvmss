<script lang="ts">
	/**
	 * The "needs attention" block at the top of the admin dashboard. The server
	 * computes the alerts; each row links to the page where the cause is fixed.
	 * Callers render it only when an alert is critical or warning; info rows
	 * are listed alongside those.
	 */
	import Card from '#lib/shared/ui/Card.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { alertHref, alertMessage, type DashboardAlert } from './dashboard-alerts';

	interface Props {
		alerts: DashboardAlert[];
	}

	let { alerts }: Props = $props();

	const SEVERITY_DOT: Record<DashboardAlert['severity'], string> = {
		critical: 'bg-destructive',
		warning: 'bg-warning',
		info: 'bg-muted-foreground'
	};

	function severityLabel(severity: DashboardAlert['severity']): string {
		switch (severity) {
			case 'critical':
				return m['admin.dashboard.severityCritical']();
			case 'warning':
				return m['admin.dashboard.severityWarning']();
			case 'info':
				return m['admin.dashboard.severityInfo']();
		}
	}
</script>

<Card pad="none" title={m['admin.dashboard.attentionTitle']()} titleId="dashboard-attention-heading">
	<ul class="divide-y divide-border" aria-labelledby="dashboard-attention-heading">
		{#each alerts as alert (`${alert.kind}/${alert.cluster}/${alert.subject ?? ''}`)}
			<li>
				<a
					href={alertHref(alert)}
					class="flex items-center gap-3 px-5 py-3 text-sm hover:bg-muted/60 pv-focus"
					data-testid="dashboard-alert"
					data-kind={alert.kind}
					data-severity={alert.severity}
				>
					<span
						class="h-2 w-2 shrink-0 rounded-full {SEVERITY_DOT[alert.severity]}"
						aria-hidden="true"
					></span>
					<span class="sr-only">
						{severityLabel(alert.severity)}:
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
</Card>
