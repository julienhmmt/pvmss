<script lang="ts">
	/**
	 * The per-pool caps on the selected cluster, under the global quota. Each
	 * row links to the pool's detail page, where the override is edited.
	 */
	import { resolve } from '$app/paths';
	import type { QuotaOverride } from '#lib/features/quota-overrides/quota-overrides.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		overrides: QuotaOverride[];
		error: string | null;
	}

	let { overrides, error }: Props = $props();
</script>

<div class="border-t border-border px-6 pb-5 pt-4" data-testid="quota-overrides">
	<h3 class="text-sm font-medium text-foreground">{m['policy.overrides.title']()}</h3>
	<p class="mt-1 text-xs text-muted-foreground">{m['policy.overrides.description']()}</p>

	{#if error}
		<div class="mt-3"><Alert>{error}</Alert></div>
	{:else if overrides.length === 0}
		<p class="mt-3 text-sm text-muted-foreground" data-testid="quota-overrides-empty">{m['policy.overrides.empty']()}</p>
	{:else}
		<ul class="mt-3 divide-y divide-border text-sm">
			{#each overrides as override (override.pool)}
				<li class="flex items-center justify-between gap-3 py-2">
					<a
						class="font-mono font-medium text-primary hover:underline"
						href="{resolve('/admin/pools/[name]', { name: override.pool })}?cluster={encodeURIComponent(override.cluster)}"
					>{override.pool}</a>
					<span class="font-mono tabular-nums">{m['policy.overrides.maxVms']({ maxVms: override.maxVms })}</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>
