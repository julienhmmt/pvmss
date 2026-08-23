<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: () => string;
		description: () => string;
		icon: Snippet;
		headingLevel?: 'h2' | 'h3';
	}

	let { title, description, icon, headingLevel = 'h3' }: Props = $props();
</script>

<div class="rounded-lg border border-border bg-card p-5 text-card-foreground">
	<div class="mb-3 inline-flex rounded-xl bg-primary/10 p-2.5 text-primary">
		{@render icon()}
	</div>
	<svelte:element this={headingLevel} class="text-base font-semibold tracking-tight">
		{title()}
	</svelte:element>
	<p class="mt-1.5 text-sm text-muted-foreground">{description()}</p>
</div>
