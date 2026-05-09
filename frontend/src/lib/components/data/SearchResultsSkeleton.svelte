<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';

	let { rows = 6 }: { rows?: number } = $props();
</script>

<div class="space-y-3">
	{#each Array(rows) as _}
		<div class="flex items-center gap-3 p-3 rounded-lg border border-border">
			<Skeleton class="h-10 w-10 rounded-md" />
			<div class="flex-1 space-y-2">
				<Skeleton class="h-4 w-1/3" />
				<Skeleton class="h-3 w-1/2" />
			</div>
			<Skeleton class="h-6 w-16 rounded-full" />
		</div>
	{/each}
</div>
