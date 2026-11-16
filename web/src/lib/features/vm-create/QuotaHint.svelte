<script lang="ts">
	/**
	 * QuotaHint - the pool allowance up front ("this will be machine 4 of 6"),
	 * so a user sees what the creation costs before the summary step.
	 * Hidden for unlimited pools and when the pool is already full (the
	 * wizard's own blocked state explains that case).
	 */
	import AllowanceMeter from '#lib/shared/ui/AllowanceMeter.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { CatalogQuota } from './create.svelte.js';

	let { quota }: { quota: CatalogQuota | null | undefined } = $props();

	const next = $derived(quota ? quota.used + 1 : 0);
	const visible = $derived(quota != null && quota.allowed > 0 && next <= quota.allowed);
</script>

{#if visible && quota}
	<div class="grid gap-2 rounded-lg border border-border bg-muted/40 p-3" data-testid="quota-hint">
		<p class="text-sm text-muted-foreground tabular-nums">
			{m['vms.create.quotaHint']({ next, allowed: quota.allowed, left: quota.allowed - next })}
		</p>
		<!-- Segments stop reading as a meter past a couple dozen. -->
		{#if quota.allowed <= 24}
			<AllowanceMeter used={next} limit={quota.allowed} label={m['vms.create.yourCapacity']()} />
		{/if}
	</div>
{/if}
