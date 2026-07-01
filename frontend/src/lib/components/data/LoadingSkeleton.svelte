<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	interface Props {
		rows?: number;
		variant?: 'table' | 'card' | 'form';
	}

	let { rows = 5, variant = 'table' }: Props = $props();
</script>

{#if variant === 'card'}
	<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
		{#each Array(rows) as _, i (i)}
			<Skeleton class="h-[120px] w-full rounded-lg" />
		{/each}
	</div>
{:else if variant === 'form'}
	<div class="space-y-4">
		{#each Array(rows) as _, i (i)}
			<div class="space-y-2">
				<Skeleton class="h-4 w-24" />
				<Skeleton class="h-10 w-full" />
			</div>
		{/each}
	</div>
{:else}
	<div class="space-y-2">
		<Skeleton class="h-10 w-full" />
		{#each Array(rows) as _, i (i)}
			<Skeleton class="h-12 w-full" />
		{/each}
	</div>
{/if}
