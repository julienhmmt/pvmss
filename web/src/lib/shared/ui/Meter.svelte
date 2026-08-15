<script lang="ts">
	/**
	 * Meter — Layer B thin quota / usage bar (mockup `.mt`). Consumes
	 * `quotaMeterView` so the unlimited / exhausted / unavailable rules live
	 * in one tested place. `role="meter"` with now/min/max only when bounded;
	 * unlimited and unavailable render text only — no fake 0–100 bar.
	 */
	import { quotaMeterView } from './quota-meter.svelte';
	import type { VmQuota } from '$lib/features/vms/list.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		quota: VmQuota | null | undefined;
		/** Optional caption rendered above the bar (e.g. "Your quota"). */
		heading?: string;
	}

	let { quota, heading }: Props = $props();

	const view = $derived(quotaMeterView(quota));

	const label = $derived.by(() => {
		switch (view.state) {
			case 'unlimited':
				return m['chrome.sidebar.quotaUnlimited']({ used: view.used ?? 0 });
			case 'exhausted':
				return m['chrome.sidebar.quotaExhausted']();
			case 'unavailable':
				return m['chrome.sidebar.quotaUnavailable']();
			default:
				return m['chrome.sidebar.quotaUsed']({ used: view.used ?? 0, allowed: view.allowed ?? 0 });
		}
	});
</script>

<div class="flex flex-col gap-1.5">
	{#if heading}<p class="text-xs font-medium text-muted-foreground">{heading}</p>{/if}
	{#if view.bounded}
		<div
			class="h-1.5 w-full overflow-hidden rounded-full bg-muted"
			role="meter"
			aria-valuemin={0}
			aria-valuemax={view.allowed ?? 100}
			aria-valuenow={view.used ?? 0}
			aria-label={label}
		>
			<div
				class="h-full rounded-full bg-primary transition-[width] motion-reduce:transition-none"
				style="width: {view.percent}%"
			></div>
		</div>
	{/if}
	<p class="text-xs {view.state === 'unavailable' ? 'text-muted-foreground-subtle' : 'text-muted-foreground'}">
		{label}
	</p>
</div>
