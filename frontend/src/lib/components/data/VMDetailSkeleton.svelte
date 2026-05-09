<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
</script>

<div class="space-y-6">
	<div class="flex items-center gap-4">
		<Skeleton class="h-12 w-12 rounded-lg" />
		<div class="flex-1 space-y-2">
			<Skeleton class="h-6 w-64" />
			<Skeleton class="h-4 w-40" />
		</div>
		<Skeleton class="h-9 w-28 rounded-md" />
	</div>

	<div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
		{#each Array(4) as _}
			<Skeleton class="h-20 w-full rounded-lg" />
		{/each}
	</div>

	<div class="flex gap-2 border-b border-border">
		{#each Array(5) as _}
			<Skeleton class="h-9 w-24" />
		{/each}
	</div>

	<div class="space-y-3">
		{#each Array(6) as _}
			<Skeleton class="h-12 w-full" />
		{/each}
	</div>
</div>
